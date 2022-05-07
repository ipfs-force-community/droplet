package idxprov

import (
	"context"
	provider "github.com/filecoin-project/index-provider"
	"github.com/filecoin-project/index-provider/engine"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log/v2"
)

type IdxProv struct {
	fx.In
	fx.Lifecycle
	Datastore badger.MetadataDS
}

var log = logging.Logger("index-provider")

func IndexProviderEngine(cfg config.IndexProviderConfig) func(params IdxProv, marketHost host.Host, dt network.ProviderDataTransfer, ps *pubsub.PubSub, nn types.NetworkName) (*engine.Engine, error) {
	return func(args IdxProv, marketHost host.Host, dt network.ProviderDataTransfer, ps *pubsub.PubSub, nn types.NetworkName) (*engine.Engine, error) {
		topicName := cfg.TopicName
		// If indexer topic name is left empty, infer it from the network name.
		if topicName == "" {
			// Use the same mechanism as the Dependency Injection (DI) to construct the topic name,
			// so that we are certain it is consistent with the name allowed by the subscription
			// filter.
			//
			// See: lp2p.GossipSub.
			topicName = network.IndexerIngestTopic(nn)
			log.Debugw("Inferred indexer topic from network name", "topic", topicName)
		}

		ipds := namespace.Wrap(args.Datastore, datastore.NewKey("/index-provider"))
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
			t, err := ps.Join(topicName)
			if err != nil {
				llog.Errorw("Failed to join indexer topic", "err", err)
				return nil, xerrors.Errorf("joining indexer topic %s: %w", topicName, err)
			}

			// Get the miner ID and set as extra gossip data.
			// The extra data is required by the lotus-specific index-provider gossip message validators.
			//ma := address.Address(maddr)
			opts = append(opts,
				engine.WithPublisherKind(engine.DataTransferPublisher),
				engine.WithDataTransfer(dt),
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

		args.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				// Note that the OnStart context is cancelled after startup. Its use in e.Start is
				// to start up gossipsub publishers and restore cache, all of  which are completed
				// before e.Start returns. Therefore, it is fine to reuse the give context.
				if err := e.Start(ctx); err != nil {
					return xerrors.Errorf("starting indexer provider engine: %w", err)
				}
				log.Infof("Started index provider engine")
				return nil
			},
			OnStop: func(_ context.Context) error {
				if err := e.Shutdown(); err != nil {
					return xerrors.Errorf("shutting down indexer provider engine: %w", err)
				}
				return nil
			},
		})
		return e, nil
	}
}

func IndexProvider(e *engine.Engine) provider.Interface {
	return e
}
