package network

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/utils"
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
	DefaultTransportsKey = utils.Special{0} // Libp2p option
	DiscoveryHandlerKey  = utils.Special{2} // Private type
	AddrsFactoryKey      = utils.Special{3} // Libp2p option
	SmuxTransportKey     = utils.Special{4} // Libp2p option
	RelayKey             = utils.Special{5} // Libp2p option
	SecurityKey          = utils.Special{6} // Libp2p option
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

var NetworkOpts = func(cfg *config.MarketConfig) utils.Option {
	return utils.Options(
		utils.Override(new(host.Host), Host),
		//libp2p
		utils.Override(new(crypto.PrivKey), PrivKey),
		utils.Override(new(crypto.PubKey), crypto.PrivKey.GetPublic),
		utils.Override(new(peer.ID), peer.IDFromPublicKey),
		utils.Override(new(peerstore.Peerstore), pstoremem.NewPeerstore),
		utils.Override(PstoreAddSelfKeysKey, PstoreAddSelfKeys),
		utils.Override(StartListeningKey, StartListening(cfg.Libp2p.ListenAddresses)),
		utils.Override(AddrsFactoryKey, AddrsFactory(cfg.Libp2p.AnnounceAddresses, cfg.Libp2p.NoAnnounceAddresses)),
		utils.Override(DefaultTransportsKey, DefaultTransports),
		utils.Override(SmuxTransportKey, SmuxTransport(true)),
		utils.Override(RelayKey, NoRelay()),
		utils.Override(SecurityKey, Security(true, false)),
		// Markets
		utils.Override(new(StagingGraphsync), NewStagingGraphsync(cfg.SimultaneousTransfers)),
	)
}
