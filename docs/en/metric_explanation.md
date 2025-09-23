# Metric Explanation

This document briefly describes the meaning of each metric.

## Graphsync Connection Status

```go
// Number of peers currently sending data to this node
GraphsyncReceivingPeersCount   = stats.Int64("graphsync/receiving_peers", "number of peers we are receiving graphsync data from", stats.UnitDimensionless)
// Number of peers currently receiving data from this node
GraphsyncSendingPeersCount     = stats.Int64("graphsync/sending_peers", "number of peers we are sending graphsync data to", stats.UnitDimensionless)
// Number of active receiving connections
GraphsyncReceivingActiveCount  = stats.Int64("graphsync/receiving_active", "number of active receiving graphsync transfers", stats.UnitDimensionless)
// Number of pending receiving connections
GraphsyncReceivingCountCount   = stats.Int64("graphsync/receiving_pending", "number of pending receiving graphsync transfers", stats.UnitDimensionless)
// Number of active sending connections
GraphsyncSendingActiveCount    = stats.Int64("graphsync/sending_active", "number of active sending graphsync transfers", stats.UnitDimensionless)
// Number of pending sending connections
GraphsyncSendingCountCount     = stats.Int64("graphsync/sending_pending", "number of pending sending graphsync transfers", stats.UnitDimensionless)
```

## Memory Usage

```go
// Amount of memory blocks allocated for receiving data
GraphsyncReceivingTotalMemoryAllocated    = stats.Int64("graphsync/receiving_total_allocated", "amount of block memory allocated for receiving graphsync data", stats.UnitBytes)
// Amount of memory blocks on hold waiting to be released for receiving data
GraphsyncReceivingTotalPendingAllocations = stats.Int64("graphsync/receiving_pending_allocations", "amount of block memory on hold being received pending allocation", stats.UnitBytes)
// Number of peers blocked from sending more data due to insufficient receiving memory
GraphsyncReceivingPeersPending            = stats.Int64("graphsync/receiving_peers_pending", "number of peers we can't receive more data from cause of pending allocations", stats.UnitDimensionless)

// Amount of memory blocks allocated for sending data
GraphsyncSendingTotalMemoryAllocated    = stats.Int64("graphsync/sending_total_allocated", "amount of block memory allocated for sending graphsync data", stats.UnitBytes)
// Amount of memory blocks on hold waiting to be released for sending data
GraphsyncSendingTotalPendingAllocations = stats.Int64("graphsync/sending_pending_allocations", "amount of block memory on hold from sending pending allocation", stats.UnitBytes)
// Number of peers blocked from receiving more data due to insufficient sending memory
GraphsyncSendingPeersPending            = stats.Int64("graphsync/sending_peers_pending", "number of peers we can't send more data to cause of pending allocations", stats.UnitDimensionless)
```

## DagStore Status

```go
// Number of Retrieval deals in DagStore
DagStorePRInitCount      = stats.Int64("dagstore/pr_init_count", "Retrieval init count", stats.UnitDimensionless)
// Total storage capacity requested by Retrieval in DagStore
DagStorePRBytesRequested = stats.Int64("dagstore/pr_requested_bytes", "Retrieval requested bytes", stats.UnitBytes)
// Number of active shards in DagStore
DagStoreActiveShardCount = stats.Int64("dagstore/active_shard_count", "Active shard count", stats.UnitMilliseconds)
```

## Piecestore Status

```go
// Number of times a retrieval deal hits an existing piece in piecestore
StorageRetrievalHitCount = stats.Int64("piecestorage/retrieval_hit", "PieceStorage hit count for retrieval", stats.UnitDimensionless)
// Number of times saving a piece hits an existing piece in piecestore
StorageSaveHitCount      = stats.Int64("piecestorage/save_hit", "PieceStorage hit count for save piece data", stats.UnitDimensionless)
```

### RPC

```go
# Total number of invalid RPC method calls
RPCInvalidMethod = stats.Int64("rpc/invalid_method", "Total number of invalid RPC methods called", stats.UnitDimensionless)
# Total number of RPC request failures
RPCRequestError  = stats.Int64("rpc/request_error", "Total number of request errors handled", stats.UnitDimensionless)
# Total number of RPC response failures
RPCResponseError = stats.Int64("rpc/response_error", "Total number of responses errors handled", stats.UnitDimensionless)
```

### Spark Deal Index

```go
# Number of eligible deals for Spark
SparkEligibleDealCount = stats.Int64("spark_eligible_deal_count", "Spark eligible deal count", stats.UnitDimensionless)
# Retrieval success rate for Spark
SparkRetrievalRate     = stats.Int64("spark_retrieval_rate", "Spark retrieval rate", stats.UnitDimensionless)
```
