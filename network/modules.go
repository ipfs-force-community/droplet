package network

import (
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	graphsync "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"go.uber.org/fx"
)

//nolint:golint
var (
	DefaultTransportsKey = builder.Special{0} // Libp2p option
	DiscoveryHandlerKey  = builder.Special{2} // Private type
	AddrsFactoryKey      = builder.Special{3} // Libp2p option
	SmuxTransportKey     = builder.Special{4} // Libp2p option
	RelayKey             = builder.Special{5} // Libp2p option
	SecurityKey          = builder.Special{6} // Libp2p option
)

const (
	PstoreAddSelfKeysKey = "PstoreAddSelfKeysKey"
	StartListeningKey    = "StartListeningKey"
)

// StagingGraphsync creates a graphsync instance which reads and writes blocks
// to the StagingBlockstore
func NewStagingGraphsync(parallelTransfers uint64) func(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs models.StagingBlockstore, h host.Host) StagingGraphsync {
	return func(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs models.StagingBlockstore, h host.Host) StagingGraphsync {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		loader := storeutil.LoaderForBlockstore(ibs)
		storer := storeutil.StorerForBlockstore(ibs)
		gs := graphsync.New(metrics.LifecycleCtx(mctx, lc), graphsyncNetwork, loader, storer, graphsync.RejectAllRequestsByDefault(), graphsync.MaxInProgressRequests(parallelTransfers))

		return gs
	}
}

var NetworkOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(host.Host), Host),
		//libp2p
		builder.Override(new(crypto.PrivKey), PrivKey),
		builder.Override(new(crypto.PubKey), crypto.PrivKey.GetPublic),
		builder.Override(new(peer.ID), peer.IDFromPublicKey),
		builder.Override(new(peerstore.Peerstore), pstoremem.NewPeerstore),
		builder.Override(PstoreAddSelfKeysKey, PstoreAddSelfKeys),
		builder.Override(StartListeningKey, StartListening(cfg.Libp2p.ListenAddresses)),
		builder.Override(AddrsFactoryKey, AddrsFactory(cfg.Libp2p.AnnounceAddresses, cfg.Libp2p.NoAnnounceAddresses)),
		builder.Override(DefaultTransportsKey, DefaultTransports),
		builder.Override(SmuxTransportKey, SmuxTransport(true)),
		builder.Override(RelayKey, NoRelay()),
		builder.Override(SecurityKey, Security(true, false)),
		// Markets
		builder.Override(new(StagingGraphsync), NewStagingGraphsync(cfg.SimultaneousTransfers)),
	)
}
