package idxprov

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/index-provider/engine"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/utils"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"strings"
)

type IdxProvIn struct {
	fx.In
	fx.Lifecycle
	Datastore badger.MetadataDS

	Host host.Host

	FullNodeAPI v1api.FullNode

	Dt network.ProviderDataTransfer
	Ps *pubsub.PubSub
	Nn types.NetworkName

	Repo     repo.Repo
	DAGStore stores.DAGStoreWrapper
}

func NewIndexProvider(cfg config.IndexProviderConfig) func(in IdxProvIn) (*IndexProvider, error) {
	return func(in IdxProvIn) (*IndexProvider, error) {
		provider, err := indexProviderEngine(in, cfg)
		if err != nil {
			return nil, err
		}

		indexProvider := &IndexProvider{
			idxProvider: provider,
			mesh:        newMeshCreator(in.FullNodeAPI, in.Host),
			dealStore:   in.Repo.StorageDealRepo(),
			dagStore:    in.DAGStore,
		}

		in.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				// Note that the OnStart context is cancelled after startup. Its use in e.Start is
				// to start up gossipsub publishers and restore cache, all of  which are completed
				// before e.Start returns. Therefore, it is fine to reuse the give context.
				if err := indexProvider.start(ctx); err != nil {
					return xerrors.Errorf("starting indexer provider engine: %w", err)
				}
				log.Infof("Started index provider engine")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := indexProvider.shutdown(ctx); err != nil {
					return xerrors.Errorf("shutting down indexer provider engine: %w", err)
				}
				return nil
			},
		})

		return indexProvider, nil
	}
}

func indexProviderEngine(in IdxProvIn, cfg config.IndexProviderConfig) (*engine.Engine, error) {
	topicName := cfg.TopicName
	// If indexer topic name is left empty, infer it from the network name.
	if topicName == "" {
		// Use the same mechanism as the Dependency Injection (DI) to construct the topic name,
		// so that we are certain it is consistent with the name allowed by the subscription
		// filter.
		//
		// See: lp2p.GossipSub.
		topicName = network.IndexerIngestTopic(in.Nn)
		log.Debugw("Inferred indexer topic from network name", "topic", topicName)
	}

	marketHost := in.Host

	ipds := namespace.Wrap(in.Datastore, datastore.NewKey("/index-provider"))
	var opts = []engine.Option{
		engine.WithDatastore(ipds),
		engine.WithHost(marketHost),
		engine.WithRetrievalAddrs(marketHost.Addrs()...),
		engine.WithEntriesCacheCapacity(cfg.EntriesCacheCapacity),
		engine.WithEntriesChunkSize(cfg.EntriesChunkSize),
		engine.WithTopicName(topicName),
		engine.WithPurgeCacheOnStart(cfg.PurgeCacheOnStart),
	}

	llog := log.With(
		"idxProvEnabled", cfg.Enable,
		"pid", marketHost.ID(),
		"topic", topicName,
		"retAddrs", marketHost.Addrs())
	// If announcements to the network are enabled, then set options for datatransfer publisher.
	if cfg.Enable {
		// Join the indexer topic using the market's pubsub instance. Otherwise, the provider
		// engine would create its own instance of pubsub down the line in go-legs, which has
		// no validators by default.
		t, err := in.Ps.Join(topicName)
		if err != nil {
			llog.Errorw("Failed to join indexer topic", "err", err)
			return nil, xerrors.Errorf("joining indexer topic %s: %w", topicName, err)
		}

		// Get the miner ID and set as extra gossip data.
		// The extra data is required by the lotus-specific index-provider gossip message validators.
		//ma := address.Address(maddr)
		opts = append(opts,
			engine.WithPublisherKind(engine.DataTransferPublisher),
			engine.WithDataTransfer(in.Dt),
			//engine.WithExtraGossipData(ma.Bytes()),
			engine.WithTopic(t),
		)
		//llog = llog.With("extraGossipData", ma, "publisher", "data-transfer")
	} else {
		opts = append(opts, engine.WithPublisherKind(engine.NoPublisher))
		llog = llog.With("publisher", "none")
	}

	// Instantiate the index provider engine.
	e, err := engine.New(opts...)
	if err != nil {
		return nil, xerrors.Errorf("creating indexer provider engine: %w", err)
	}
	llog.Info("Instantiated index provider engine")

	showHostAddresses(in.Host)

	//in.Lifecycle.Append(fx.Hook{
	//	OnStart: func(ctx context.Context) error {
	//		// Note that the OnStart context is cancelled after startup. Its use in e.Start is
	//		// to start up gossipsub publishers and restore cache, all of  which are completed
	//		// before e.Start returns. Therefore, it is fine to reuse the give context.
	//		if err := e.Start(ctx); err != nil {
	//			return xerrors.Errorf("starting indexer provider engine: %w", err)
	//		}
	//		log.Infof("Started index provider engine")
	//		return nil
	//	},
	//	OnStop: func(_ context.Context) error {
	//		if err := e.Shutdown(); err != nil {
	//			return xerrors.Errorf("shutting down indexer provider engine: %w", err)
	//		}
	//		return nil
	//	},
	//})
	return e, nil
}

func showHostAddresses(h host.Host) {
	addrInfos := utils.HostConnectedPeers(h)
	sb := &strings.Builder{}
	for _, info := range addrInfos {
		sb.WriteString(fmt.Sprintf("%s\n", info.String()))
	}
	log.Infof("connected peers:\n%s", sb.String())
}
