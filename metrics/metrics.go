package metrics

import (
	rpcMetrics "github.com/filecoin-project/go-jsonrpc/metrics"
	"github.com/ipfs-force-community/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Global Tags
var (
	StorageNameTag, _ = tag.NewKey("storage")
	StatusTag, _      = tag.NewKey("status")

	MinerAddressTag, _ = tag.NewKey("miner")
)

const (
	StatusOK  = "OK"
	StatusErr = "ERR"
)

// Distribution
var defaultMillisecondsDistribution = view.Distribution(100, 500, 1000, 3000, 5000, 8000, 10000, 15000, 30000, 60000)

var (
	RetrievalTransferEvent = metrics.NewCounterWithCategory("retrieval/transfer_event", "retrieval transfer event")
	ShardNum               = metrics.NewInt64WithCategory("shard/num", "shard num in different state", "")
)

var (
	GraphsyncReceivingPeersCount              = stats.Int64("graphsync/receiving_peers", "number of peers we are receiving graphsync data from", stats.UnitDimensionless)
	GraphsyncReceivingActiveCount             = stats.Int64("graphsync/receiving_active", "number of active receiving graphsync transfers", stats.UnitDimensionless)
	GraphsyncReceivingCountCount              = stats.Int64("graphsync/receiving_pending", "number of pending receiving graphsync transfers", stats.UnitDimensionless)
	GraphsyncReceivingTotalMemoryAllocated    = stats.Int64("graphsync/receiving_total_allocated", "amount of block memory allocated for receiving graphsync data", stats.UnitBytes)
	GraphsyncReceivingTotalPendingAllocations = stats.Int64("graphsync/receiving_pending_allocations", "amount of block memory on hold being received pending allocation", stats.UnitBytes)
	GraphsyncReceivingPeersPending            = stats.Int64("graphsync/receiving_peers_pending", "number of peers we can't receive more data from cause of pending allocations", stats.UnitDimensionless)

	GraphsyncSendingPeersCount              = stats.Int64("graphsync/sending_peers", "number of peers we are sending graphsync data to", stats.UnitDimensionless)
	GraphsyncSendingActiveCount             = stats.Int64("graphsync/sending_active", "number of active sending graphsync transfers", stats.UnitDimensionless)
	GraphsyncSendingCountCount              = stats.Int64("graphsync/sending_pending", "number of pending sending graphsync transfers", stats.UnitDimensionless)
	GraphsyncSendingTotalMemoryAllocated    = stats.Int64("graphsync/sending_total_allocated", "amount of block memory allocated for sending graphsync data", stats.UnitBytes)
	GraphsyncSendingTotalPendingAllocations = stats.Int64("graphsync/sending_pending_allocations", "amount of block memory on hold from sending pending allocation", stats.UnitBytes)
	GraphsyncSendingPeersPending            = stats.Int64("graphsync/sending_peers_pending", "number of peers we can't send more data to cause of pending allocations", stats.UnitDimensionless)

	DagStorePRInitCount      = stats.Int64("dagstore/pr_init_count", "Retrieval init count", stats.UnitDimensionless)
	DagStorePRBytesRequested = stats.Int64("dagstore/pr_requested_bytes", "Retrieval requested bytes", stats.UnitBytes)

	DagStoreLoadShard        = stats.Int64("dagstore/load_shard", "Load shard", stats.UnitMilliseconds)
	DagStoreActiveShardCount = stats.Int64("dagstore/active_shard_count", "Active shard count", stats.UnitMilliseconds)

	ActiveDealCount = stats.Int64("active_deal_count", "Active deal count", stats.UnitMilliseconds)

	SparkEligibleDealCount = stats.Int64("spark_eligible_deal_count", "Spark eligible deal count", stats.UnitDimensionless)
	SparkRetrievalRate     = stats.Int64("spark_retrieval_rate", "Spark retrieval rate", stats.UnitDimensionless)

	StorageRetrievalHitCount = stats.Int64("piecestorage/retrieval_hit", "PieceStorage hit count for retrieval", stats.UnitDimensionless)
	StorageSaveHitCount      = stats.Int64("piecestorage/save_hit", "PieceStorage hit count for save piece data", stats.UnitDimensionless)
)

var (
	// graphsync
	GraphsyncReceivingPeersCountView = &view.View{
		Measure:     GraphsyncReceivingPeersCount,
		Aggregation: view.LastValue(),
	}
	GraphsyncReceivingActiveCountView = &view.View{
		Measure:     GraphsyncReceivingActiveCount,
		Aggregation: view.LastValue(),
	}
	GraphsyncReceivingCountCountView = &view.View{
		Measure:     GraphsyncReceivingCountCount,
		Aggregation: view.LastValue(),
	}
	GraphsyncReceivingTotalMemoryAllocatedView = &view.View{
		Measure:     GraphsyncReceivingTotalMemoryAllocated,
		Aggregation: view.LastValue(),
	}
	GraphsyncReceivingTotalPendingAllocationsView = &view.View{
		Measure:     GraphsyncReceivingTotalPendingAllocations,
		Aggregation: view.LastValue(),
	}
	GraphsyncReceivingPeersPendingView = &view.View{
		Measure:     GraphsyncReceivingPeersPending,
		Aggregation: view.LastValue(),
	}
	GraphsyncSendingPeersCountView = &view.View{
		Measure:     GraphsyncSendingPeersCount,
		Aggregation: view.LastValue(),
	}
	GraphsyncSendingActiveCountView = &view.View{
		Measure:     GraphsyncSendingActiveCount,
		Aggregation: view.LastValue(),
	}
	GraphsyncSendingCountCountView = &view.View{
		Measure:     GraphsyncSendingCountCount,
		Aggregation: view.LastValue(),
	}
	GraphsyncSendingTotalMemoryAllocatedView = &view.View{
		Measure:     GraphsyncSendingTotalMemoryAllocated,
		Aggregation: view.LastValue(),
	}
	GraphsyncSendingTotalPendingAllocationsView = &view.View{
		Measure:     GraphsyncSendingTotalPendingAllocations,
		Aggregation: view.LastValue(),
	}
	GraphsyncSendingPeersPendingView = &view.View{
		Measure:     GraphsyncSendingPeersPending,
		Aggregation: view.LastValue(),
	}

	// dagstore
	DagStorePRInitCountView = &view.View{
		Measure:     DagStorePRInitCount,
		Aggregation: view.Count(),
	}
	DagStorePRBytesRequestedView = &view.View{
		Measure:     DagStorePRBytesRequested,
		Aggregation: view.Sum(),
	}

	DagStoreLoadShardView = &view.View{
		Measure:     DagStoreLoadShard,
		TagKeys:     []tag.Key{StatusTag},
		Aggregation: defaultMillisecondsDistribution,
	}
	DagStoreActiveShardCountView = &view.View{
		Measure:     DagStoreActiveShardCount,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{MinerAddressTag},
	}

	ActiveDealCountView = &view.View{
		Measure:     ActiveDealCount,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{MinerAddressTag},
	}

	SparkRetrievalRateView = &view.View{
		Measure:     SparkRetrievalRate,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{MinerAddressTag},
	}
	SparkEligibleDealCountView = &view.View{
		Measure:     SparkEligibleDealCount,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{MinerAddressTag},
	}

	// piece storage
	StorageRetrievalHitCountView = &view.View{
		Measure:     StorageRetrievalHitCount,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{StorageNameTag},
	}
	StorageSaveHitCountView = &view.View{
		Measure:     StorageSaveHitCount,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{StorageNameTag},
	}
)

var views = append([]*view.View{
	GraphsyncReceivingPeersCountView,
	GraphsyncReceivingActiveCountView,
	GraphsyncReceivingCountCountView,
	GraphsyncReceivingTotalMemoryAllocatedView,
	GraphsyncReceivingTotalPendingAllocationsView,
	GraphsyncReceivingPeersPendingView,
	GraphsyncSendingPeersCountView,
	GraphsyncSendingActiveCountView,
	GraphsyncSendingCountCountView,
	GraphsyncSendingTotalMemoryAllocatedView,
	GraphsyncSendingTotalPendingAllocationsView,
	GraphsyncSendingPeersPendingView,

	DagStorePRInitCountView,
	DagStorePRBytesRequestedView,
	DagStoreLoadShardView,
	DagStoreActiveShardCountView,

	ActiveDealCountView,
	SparkRetrievalRateView,
	SparkEligibleDealCountView,

	StorageRetrievalHitCountView,
	StorageSaveHitCountView,
}, rpcMetrics.DefaultViews...)

func init() {
	for _, v := range views {
		if err := view.Register(v); err != nil {
			panic(err)
		}
	}
}
