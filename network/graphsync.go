package network

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/filecoin-project/venus-market/v2/config"
	marketMetrics "github.com/filecoin-project/venus-market/v2/metrics"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/libp2p/go-libp2p-core/host"
	"go.opencensus.io/stats"
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
		graphsyncStats(mctx, lc, gs)
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

		graphsyncStats(mctx, lc, gs)
		return gs
	}
}

func graphsyncStats(mctx metrics.MetricsCtx, lc fx.Lifecycle, gs graphsync.GraphExchange) {
	stopStats := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				t := time.NewTicker(10 * time.Second)
				defer t.Stop()

				for {
					select {
					case <-t.C:

						st := gs.Stats()
						stats.Record(mctx, marketMetrics.GraphsyncReceivingPeersCount.M(int64(st.OutgoingRequests.TotalPeers)))
						stats.Record(mctx, marketMetrics.GraphsyncReceivingActiveCount.M(int64(st.OutgoingRequests.Active)))
						stats.Record(mctx, marketMetrics.GraphsyncReceivingCountCount.M(int64(st.OutgoingRequests.Pending)))
						stats.Record(mctx, marketMetrics.GraphsyncReceivingTotalMemoryAllocated.M(int64(st.IncomingResponses.TotalAllocatedAllPeers)))
						stats.Record(mctx, marketMetrics.GraphsyncReceivingTotalPendingAllocations.M(int64(st.IncomingResponses.TotalPendingAllocations)))
						stats.Record(mctx, marketMetrics.GraphsyncReceivingPeersPending.M(int64(st.IncomingResponses.NumPeersWithPendingAllocations)))
						stats.Record(mctx, marketMetrics.GraphsyncSendingPeersCount.M(int64(st.IncomingRequests.TotalPeers)))
						stats.Record(mctx, marketMetrics.GraphsyncSendingActiveCount.M(int64(st.IncomingRequests.Active)))
						stats.Record(mctx, marketMetrics.GraphsyncSendingCountCount.M(int64(st.IncomingRequests.Pending)))
						stats.Record(mctx, marketMetrics.GraphsyncSendingTotalMemoryAllocated.M(int64(st.OutgoingResponses.TotalAllocatedAllPeers)))
						stats.Record(mctx, marketMetrics.GraphsyncSendingTotalPendingAllocations.M(int64(st.OutgoingResponses.TotalPendingAllocations)))
						stats.Record(mctx, marketMetrics.GraphsyncSendingPeersPending.M(int64(st.OutgoingResponses.NumPeersWithPendingAllocations)))

					case <-stopStats:
						return
					}
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(stopStats)
			return nil
		},
	})
}
