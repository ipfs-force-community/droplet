package indexprovider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	markettypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	cbor "github.com/ipfs/go-ipld-cbor"
	provider "github.com/ipni/index-provider"
	"github.com/ipni/index-provider/engine"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/fx"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/badger"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/utils"
)

type IndexProviderMgr struct {
	cfg       *config.ProviderConfig
	h         host.Host
	r         repo.Repo
	full      v1.FullNode
	dagStore  stores.DAGStoreWrapper
	topic     *pubsub.Topic
	topicName string
	ds        badger.MetadataDS

	indexProviders map[address.Address]*Wrapper
	lk             sync.Mutex
}

func NewIndexProviderMgr(lc fx.Lifecycle,
	cfg *config.MarketConfig,
	h host.Host,
	r repo.Repo,
	full v1.FullNode,
	dagStore stores.DAGStoreWrapper,
	ps *pubsub.PubSub,
	ds badger.MetadataDS,
	nn NetworkName,
) (*IndexProviderMgr, error) {
	mgr := &IndexProviderMgr{
		cfg:      cfg.CommonProvider,
		h:        h,
		r:        r,
		full:     full,
		dagStore: dagStore,
		ds:       ds,

		indexProviders: make(map[address.Address]*Wrapper),
	}

	err := mgr.setTopic(ps, string(nn))
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var minerAddrs []address.Address
			for _, miner := range cfg.Miners {
				minerAddrs = append(minerAddrs, miner.Addr.Unwrap())
			}
			if err := mgr.initAllIndexProviders(ctx, minerAddrs); err != nil {
				return err
			}
			for _, miner := range cfg.Miners {
				err := mgr.IndexAnnounceAllDeals(ctx, miner.Addr.Unwrap())
				fmt.Println("IndexAnnounceAllDeals", miner.Addr.Unwrap().String(), err)
				if err != nil {
					fmt.Printf("failed to announce all deals(%v): %v\n", miner.Addr, err)
				}
				if miner.Addr.Unwrap().String() == "f03071235" {
					mgr.IndexerAnnounceLatest(ctx, miner.Addr.Unwrap())
				}
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

func (m *IndexProviderMgr) setTopic(ps *pubsub.PubSub, nn string) error {
	topicName := m.cfg.IndexProvider.TopicName
	// If indexer topic name is left empty, infer it from the network name.
	if topicName == "" {
		// Use the same mechanism as the Dependency Injection (DI) to construct the topic name,
		// so that we are certain it is consistent with the name allowed by the subscription
		// filter.
		//
		// See: lp2p.GossipSub.
		topicName = types.IndexerIngestTopic(nn)
		log.Debugw("Inferred indexer topic from network name", "topic", topicName)
	}
	// Join the indexer topic using the market's pubsub instance. Otherwise, the provider
	// engine would create its own instance of pubsub down the line in dagsync, which has
	// no validators by default.
	t, err := ps.Join(topicName)
	if err != nil {
		return fmt.Errorf("joining indexer topic %s: %w", topicName, err)
	}

	m.topic = t
	m.topicName = topicName

	return nil
}

func (m *IndexProviderMgr) initAllIndexProviders(ctx context.Context, minerAddrs []address.Address) error {
	for _, minerAddr := range minerAddrs {
		idxProv, err := m.initIndexProvider(ctx, minerAddr)
		if err != nil {
			return fmt.Errorf("init index provider failed, miner addr: %s, err: %w", minerAddr, err)
		}
		wrapper, err := NewWrapper(m.h, m.cfg, m.full, m.r, m.dagStore, idxProv)
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
		engine.WithTopicName(m.topicName),
		engine.WithPurgeCacheOnStart(cfg.PurgeCacheOnStart),
	}

	llog := log.With(
		"idxProvEnabled", cfg.Enable,
		"pid", m.h.ID(),
		"topic", m.topicName,
		"retAddrs", m.h.Addrs())

	// If announcements to the network are enabled, then set options for the publisher.
	var e *engine.Engine
	if cfg.Enable {
		// Get the miner ID and set as extra gossip data.
		// The extra data is required by the lotus-specific index-provider gossip message validators.
		opts = append(opts,
			engine.WithTopic(m.topic),
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
		wrapper, err = NewWrapper(m.h, m.cfg, m.full, m.r, m.dagStore, idxProv)
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

	return w.AnnounceDealRemoved(ctx, propCid.Bytes())
}

func (m *IndexProviderMgr) AnnounceDirectDeal(ctx context.Context, deal *markettypes.DirectDeal) (cid.Cid, error) {
	w, err := m.GetIndexProvider(deal.Provider)
	if err != nil {
		return cid.Undef, err
	}

	return w.AnnounceDirectDeal(ctx, deal)
}

func (m *IndexProviderMgr) AnnounceDirectDealRemoved(ctx context.Context, minerAddr address.Address, dealUUID uuid.UUID) (cid.Cid, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return cid.Undef, err
	}

	contextID, err := dealUUID.MarshalBinary()
	if err != nil {
		return cid.Undef, fmt.Errorf("marshalling the deal UUID: %w", err)
	}

	return w.AnnounceDealRemoved(ctx, contextID)
}

func (m *IndexProviderMgr) MultihashLister(ctx context.Context, minerAddr address.Address, prov peer.ID, root []byte) (provider.MultihashIterator, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return nil, err
	}
	return w.MultihashLister(ctx, prov, root)
}

func (m *IndexProviderMgr) IndexerAnnounceLatest(ctx context.Context, minerAddr address.Address) (cid.Cid, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return cid.Undef, err
	}
	return w.IndexerAnnounceLatest(ctx)
}

func (m *IndexProviderMgr) IndexerAnnounceLatestHttp(ctx context.Context, minerAddr address.Address, urls []string) (cid.Cid, error) {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return cid.Undef, err
	}

	return w.IndexerAnnounceLatestHttp(ctx, urls)
}

var filterDealTimestamp = func() uint64 {
	d := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	return uint64(d.Unix())
}()

func (m *IndexProviderMgr) IndexAnnounceAllDeals(ctx context.Context, minerAddr address.Address) error {
	w, err := m.GetIndexProvider(minerAddr)
	if err != nil {
		return err
	}
	if !w.enabled {
		return errors.New("cannot announce all deals: index provider is disabled")
	}

	// activeSectors, err := m.getActiveSectors(ctx, minerAddr)
	// if err != nil {
	// 	return err
	// }

	deals, err := m.r.StorageDealRepo().ListDealByAddr(ctx, minerAddr)
	if err != nil {
		return err
	}

	merr := &multierror.Error{}
	success := 0
	for _, deal := range deals {
		if deal.State != storagemarket.StorageDealActive {
			continue
		}
		if deal.CreatedAt < uint64(filterDealTimestamp) {
			continue
		}

		// present, err := activeSectors.IsSet(uint64(deal.SectorNumber))
		// if err != nil {
		// 	return fmt.Errorf("checking if bitfield is set: %w", err)
		// }

		// if !present {
		// 	continue
		// }

		_, err = w.AnnounceDeal(ctx, deal)
		if err != nil {
			// don't log already advertised errors as errors - just skip them
			if !errors.Is(err, provider.ErrAlreadyAdvertised) {
				merr = multierror.Append(merr, err)
				log.Errorw("failed to announce deal to Indexer", "dealId", deal.ProposalCid, "err", err)
			}
			continue
		}
		success++
	}

	log.Infow("finished announcing deals to Indexer", "number of deals", success)
	dealActive := markettypes.DealActive
	directDeals, err := m.r.DirectDealRepo().ListDeal(ctx, markettypes.DirectDealQueryParams{
		Provider: minerAddr,
		State:    &dealActive,
	})
	if err != nil {
		return err
	}
	success = 0
	for _, deal := range directDeals {
		if deal.CreatedAt < uint64(filterDealTimestamp) {
			continue
		}

		_, err = m.AnnounceDirectDealRemoved(ctx, deal.Provider, deal.ID)
		if err != nil {
			panic(err)
		}

		// present, err := activeSectors.IsSet(uint64(deal.SectorID))
		// if err != nil {
		// 	return fmt.Errorf("checking if bitfield is set: %w", err)
		// }

		// if !present {
		// 	continue
		// }

		_, err = w.AnnounceDirectDeal(ctx, deal)
		if err != nil {
			// don't log already advertised errors as errors - just skip them
			if !errors.Is(err, provider.ErrAlreadyAdvertised) {
				merr = multierror.Append(merr, err)
				log.Errorw("failed to announce deal to Indexer", "dealAllocation", deal.AllocationID, "err", err)
			}
			continue
		}
		success++
	}

	log.Infow("finished announcing all direct deals to Indexer", "number of deals", success)

	return merr.ErrorOrNil()
}

func (m *IndexProviderMgr) getActiveSectors(ctx context.Context, minerAddr address.Address) (bitfield.BitField, error) {
	mActor, err := m.full.StateGetActor(ctx, minerAddr, types.EmptyTSK)
	if err != nil {
		return bitfield.BitField{}, fmt.Errorf("getting actor for the miner %s: %w", minerAddr, err)
	}

	store := adt.WrapStore(ctx, cbor.NewCborStore(blockstore.NewAPIBlockstore(m.full)))
	mas, err := miner.Load(store, mActor)
	if err != nil {
		return bitfield.BitField{}, fmt.Errorf("loading miner actor state %s: %w", minerAddr, err)
	}
	liveSectors, err := miner.AllPartSectors(mas, miner.Partition.LiveSectors)
	if err != nil {
		return bitfield.BitField{}, fmt.Errorf("getting live sector sets for miner %s: %w", minerAddr, err)
	}
	unProvenSectors, err := miner.AllPartSectors(mas, miner.Partition.UnprovenSectors)
	if err != nil {
		return bitfield.BitField{}, fmt.Errorf("getting unproven sector sets for miner %s: %w", minerAddr, err)
	}

	return bitfield.MergeBitFields(liveSectors, unProvenSectors)
}
