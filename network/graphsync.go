package network

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	"github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/fx"
)

// Graphsync creates a graphsync instance from the given loader and storer
func NewGraphsync(parallelTransfers uint64) func(mctx metrics.MetricsCtx, lc fx.Lifecycle, r *config.HomeDir, clientBs models.ClientBlockstore, h host.Host) (Graphsync, error) {
	return func(mctx metrics.MetricsCtx, lc fx.Lifecycle, r *config.HomeDir, clientBs models.ClientBlockstore, h host.Host) (Graphsync, error) {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		loader := storeutil.LoaderForBlockstore(clientBs)
		storer := storeutil.StorerForBlockstore(clientBs)

		gs := graphsyncimpl.New(metrics.LifecycleCtx(mctx, lc), graphsyncNetwork, loader, storer, graphsyncimpl.RejectAllRequestsByDefault(), graphsyncimpl.MaxInProgressRequests(parallelTransfers))
		//notice no chain data on market
		/*	chainLoader := storeutil.LoaderForBlockstore(chainBs)
			chainStorer := storeutil.StorerForBlockstore(chainBs)
			err := gs.RegisterPersistenceOption("chainstore", chainLoader, chainStorer)
			if err != nil {
				return nil, err
			}*/
		gs.RegisterIncomingRequestHook(func(p peer.ID, requestData graphsync.RequestData, hookActions graphsync.IncomingRequestHookActions) {
			_, has := requestData.Extension("chainsync")
			if has {
				// TODO: we should confirm the selector is a reasonable one before we validate
				// TODO: this code will get more complicated and should probably not live here eventually
				hookActions.ValidateRequest()
				hookActions.UsePersistenceOption("chainstore")
			}
		})
		gs.RegisterOutgoingRequestHook(func(p peer.ID, requestData graphsync.RequestData, hookActions graphsync.OutgoingRequestHookActions) {
			_, has := requestData.Extension("chainsync")
			if has {
				hookActions.UsePersistenceOption("chainstore")
			}
		})
		return gs, nil
	}
}

// StagingGraphsync creates a graphsync instance which reads and writes blocks
// to the StagingBlockstore
func NewStagingGraphsync(parallelTransfers uint64) func(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs models.StagingBlockstore, h host.Host) StagingGraphsync {
	return func(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs models.StagingBlockstore, h host.Host) StagingGraphsync {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		loader := storeutil.LoaderForBlockstore(ibs)
		storer := storeutil.StorerForBlockstore(ibs)
		gs := graphsyncimpl.New(metrics.LifecycleCtx(mctx, lc), graphsyncNetwork, loader, storer, graphsyncimpl.RejectAllRequestsByDefault(), graphsyncimpl.MaxInProgressRequests(parallelTransfers))

		return gs
	}
}
