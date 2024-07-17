package indexprovider

import (
	"context"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-address"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	markettypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/dagstore"
	"github.com/ipfs-force-community/droplet/v2/models/badger"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	provider "github.com/ipni/index-provider"
	"github.com/ipni/index-provider/engine"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"
)

type IndexProviderMgr struct {
	cfg        *config.ProviderConfig
	h          host.Host
	r          repo.Repo
	full       v1.FullNode
	dagWrapper *dagstore.Wrapper
	ps         *pubsub.PubSub
	nn         string
	ds         badger.MetadataDS

	indexProviders map[address.Address]*Wrapper
	lk             sync.Mutex
}

func NewIndexProviderMgr(lc fx.Lifecycle,
	cfg *config.MarketConfig,
	h host.Host,
	r repo.Repo,
	full v1.FullNode,
	dagWrapper *dagstore.Wrapper,
	ps *pubsub.PubSub,
	ds badger.MetadataDS,
	nn NetworkName,
) (*IndexProviderMgr, error) {
	mgr := &IndexProviderMgr{
		cfg:        cfg.CommonProvider,
		h:          h,
		r:          r,
		full:       full,
		dagWrapper: dagWrapper,
		ps:         ps,
		nn:         string(nn),
		ds:         ds,

		indexProviders: make(map[address.Address]*Wrapper),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var minerAddrs []address.Address
			for _, miner := range cfg.Miners {
				addr := miner.Addr.Unwrap()
				if addr.Empty() {
					continue
				}
				minerAddrs = append(minerAddrs, addr)
			}
			if err := mgr.initAllIndexProviders(ctx, minerAddrs); err != nil {
				return err
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := mgr.Stop(ctx); err != nil {
				return fmt.Errorf("shutting down indexer provider engine: %w", err)
			}
			return nil
		},
	})

	return mgr, nil
}

func (m *IndexProviderMgr) initAllIndexProviders(ctx context.Context, minerAddrs []address.Address) error {
	for _, minerAddr := range minerAddrs {
		idxProv, err := m.initIndexProvider(ctx, minerAddr)
		if err != nil {
			return fmt.Errorf("init index provider failed, miner addr: %s, err: %w", minerAddr, err)
		}
		wrapper, err := NewWrapper(m.h, m.cfg, m.full, m.r, m.dagWrapper, idxProv)
		if err != nil {
			return fmt.Errorf("new index provider wrapper failed, miner addr: %s, err: %w", minerAddr, err)
		}
		wrapper.Start(ctx)
		m.indexProviders[minerAddr] = wrapper
	}
	return nil
}

func (m *IndexProviderMgr) initIndexProvider(ctx context.Context, minerAddr address.Address) (provider.Interface, error) {
	cfg := m.cfg.IndexProvider
	if !cfg.Enable {
		log.Warnf("Starting with index provider disabled - no announcements will be made to the index provider")
		return NewDisabledIndexProvider(), nil
	}

	topicName := cfg.TopicName
	// If indexer topic name is left empty, infer it from the network name.
	if topicName == "" {
		// Use the same mechanism as the Dependency Injection (DI) to construct the topic name,
		// so that we are certain it is consistent with the name allowed by the subscription
		// filter.
		//
		// See: lp2p.GossipSub.
		topicName = types.IndexerIngestTopic(m.nn)
		log.Debugw("Inferred indexer topic from network name", "topic", topicName)
	}

	marketHostAddrs := m.h.Addrs()
	marketHostAddrsStr := make([]string, 0, len(marketHostAddrs))
	for _, a := range marketHostAddrs {
		marketHostAddrsStr = append(marketHostAddrsStr, a.String())
	}

	ipds := namespace.Wrap(m.ds, datastore.NewKey("/index-provider"))
	var opts = []engine.Option{
		engine.WithDatastore(ipds),
		engine.WithHost(m.h),
		engine.WithRetrievalAddrs(marketHostAddrsStr...),
		engine.WithEntriesCacheCapacity(cfg.EntriesCacheCapacity),
		engine.WithChainedEntries(cfg.EntriesChunkSize),
		engine.WithTopicName(topicName),
		engine.WithPurgeCacheOnStart(cfg.PurgeCacheOnStart),
	}

	llog := log.With(
		"idxProvEnabled", cfg.Enable,
		"pid", m.h.ID(),
		"topic", topicName,
		"retAddrs", m.h.Addrs())

	// If announcements to the network are enabled, then set options for the publisher.
	var e *engine.Engine
	if cfg.Enable {
		// Join the indexer topic using the market's pubsub instance. Otherwise, the provider
		// engine would create its own instance of pubsub down the line in dagsync, which has
		// no validators by default.
		t, err := m.ps.Join(topicName)
		if err != nil {
			llog.Errorw("Failed to join indexer topic", "err", err)
			return nil, fmt.Errorf("joining indexer topic %s: %w", topicName, err)
		}

		// Get the miner ID and set as extra gossip data.
		// The extra data is required by the lotus-specific index-provider gossip message validators.
		opts = append(opts,
			engine.WithTopic(t),
			engine.WithExtraGossipData(minerAddr.Bytes()),
		)
		if cfg.Announce.AnnounceOverHttp {
			opts = append(opts, engine.WithDirectAnnounce(cfg.Announce.DirectAnnounceURLs...))
		}

		// Advertisements can be served over HTTP or HTTP over libp2p.
		if cfg.HttpPublisher.Enabled {
			announceAddr, err := utils.ToHttpMultiaddr(cfg.HttpPublisher.PublicHostname, cfg.HttpPublisher.Port)
			if err != nil {
				return nil, fmt.Errorf("parsing HTTP Publisher hostname '%s' / port %d: %w",
					cfg.HttpPublisher.PublicHostname, cfg.HttpPublisher.Port, err)
			}
			opts = append(opts,
				engine.WithHttpPublisherListenAddr(fmt.Sprintf("0.0.0.0:%d", cfg.HttpPublisher.Port)),
				engine.WithHttpPublisherAnnounceAddr(announceAddr.String()),
			)
			if cfg.HttpPublisher.WithLibp2p {
				opts = append(opts, engine.WithPublisherKind(engine.Libp2pHttpPublisher))
				llog = llog.With("publisher", "http", "announceAddr", announceAddr)
			} else {
				opts = append(opts, engine.WithPublisherKind(engine.HttpPublisher))
				llog = llog.With("publisher", "http and libp2phttp", "announceAddr", announceAddr, "extraGossipData", minerAddr)
			}
		} else {
			// HTTP publisher not enabled, so use only libp2p
			opts = append(opts, engine.WithPublisherKind(engine.Libp2pPublisher))
			llog = llog.With("publisher", "libp2phttp", "extraGossipData", minerAddr)
		}
	} else {
		opts = append(opts, engine.WithPublisherKind(engine.NoPublisher))
		llog = llog.With("publisher", "none")
	}

	// Instantiate the index provider engine.
	var err error
	e, err = engine.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating indexer provider engine: %w", err)
	}
	llog.Info("Instantiated index provider engine")

	return e, e.Start(ctx)
}

func (m *IndexProviderMgr) Stop(ctx context.Context) error {
	for _, p := range m.indexProviders {
		p.Stop()
		if err := p.prov.Shutdown(); err != nil {
			return fmt.Errorf("closing index provider: %w", err)
		}
	}
	return nil
}

func (m *IndexProviderMgr) GetIndexProvider(minerAddr address.Address) (*Wrapper, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	wrapper, ok := m.indexProviders[minerAddr]
	if !ok {
		ctx := context.Background()
		idxProv, err := m.initIndexProvider(ctx, minerAddr)
		if err != nil {
			return nil, err
		}
		wrapper, err = NewWrapper(m.h, m.cfg, m.full, m.r, m.dagWrapper, idxProv)
		if err != nil {
			return nil, fmt.Errorf("new index provider wrapper failed, miner addr: %s, err: %w", minerAddr, err)
		}
		wrapper.Start(ctx)
		m.indexProviders[minerAddr] = wrapper
	}
	return wrapper, nil
}

func (m *IndexProviderMgr) AnnounceDeal(ctx context.Context, deal *markettypes.MinerDeal) (cid.Cid, error) {
	w, err := m.GetIndexProvider(deal.Proposal.Provider)
	if err != nil {
		return cid.Undef, err
	}
	return w.AnnounceDeal(ctx, deal)
}

func (m *IndexProviderMgr) AnnounceDealRemoved(ctx context.Context, minerAddr address.Address, propCid cid.Cid) (cid.Cid, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return cid.Undef, err
	}

	return w.AnnounceDealRemoved(ctx, propCid)
}

func (m *IndexProviderMgr) AnnounceDirectDeal(ctx context.Context, minerAddr address.Address, entry *markettypes.DirectDeal) (cid.Cid, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return cid.Undef, err
	}

	return w.AnnounceDirectDeal(ctx, entry)
}

func (m *IndexProviderMgr) AnnounceDirectDealRemoved(ctx context.Context, minerAddr address.Address, dealUUID uuid.UUID) (cid.Cid, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return cid.Undef, err
	}

	return w.AnnounceDirectDealRemoved(ctx, dealUUID)
}
