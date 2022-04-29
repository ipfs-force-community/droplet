package network

import (
	"os"
	"strconv"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"
)

// MaxTraversalLinks configures the maximum number of links to traverse in a DAG while calculating
// CommP and traversing a DAG with graphsync; invokes a budget on DAG depth and density.
var MaxTraversalLinks uint64 = 32 * (1 << 20)

func init() {
	if envMaxTraversal, err := strconv.ParseUint(os.Getenv("VENUS_MAX_TRAVERSAL_LINKS"), 10, 64); err == nil {
		MaxTraversalLinks = envMaxTraversal
	}
}

// Graphsync creates a graphsync instance from the given loader and storer
func NewGraphsync(simultaneousTransfersForRetrieval, simultaneousTransfersForStorage uint64) func(mctx metrics.MetricsCtx, lc fx.Lifecycle, r *config.HomeDir, clientBs badger.ClientBlockstore, h host.Host) (Graphsync, error) {
	return func(mctx metrics.MetricsCtx, lc fx.Lifecycle, r *config.HomeDir, clientBs badger.ClientBlockstore, h host.Host) (Graphsync, error) {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		lsys := storeutil.LinkSystemForBlockstore(clientBs)

		gs := graphsyncimpl.New(metrics.LifecycleCtx(mctx, lc), graphsyncNetwork, lsys,
			graphsyncimpl.RejectAllRequestsByDefault(),
			graphsyncimpl.MaxInProgressIncomingRequests(simultaneousTransfersForStorage),
			graphsyncimpl.MaxInProgressOutgoingRequests(simultaneousTransfersForRetrieval),
			graphsyncimpl.MaxLinksPerIncomingRequests(MaxTraversalLinks),
			graphsyncimpl.MaxLinksPerOutgoingRequests(MaxTraversalLinks),
		)
		return gs, nil
	}
}

// StagingGraphsync creates a graphsync instance which reads and writes blocks
// to the StagingBlockstore
func NewStagingGraphsync(simultaneousTransfersForRetrieval, simultaneousTransfersForStoragePerClient, simultaneousTransfersForStorage uint64) func(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs badger.StagingBlockstore, h host.Host) StagingGraphsync {
	return func(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs badger.StagingBlockstore, h host.Host) StagingGraphsync {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		lsys := storeutil.LinkSystemForBlockstore(ibs)
		gs := graphsyncimpl.New(metrics.LifecycleCtx(mctx, lc), graphsyncNetwork, lsys,
			graphsyncimpl.RejectAllRequestsByDefault(),
			graphsyncimpl.MaxInProgressIncomingRequests(simultaneousTransfersForRetrieval),
			//graphsyncimpl.MaxInProgressIncomingRequestsPerPeer(simultaneousTransfersForStoragePerClient),
			graphsyncimpl.MaxInProgressOutgoingRequests(simultaneousTransfersForStorage),
			graphsyncimpl.MaxLinksPerIncomingRequests(MaxTraversalLinks),
			graphsyncimpl.MaxLinksPerOutgoingRequests(MaxTraversalLinks),
		)

		return gs
	}
}
