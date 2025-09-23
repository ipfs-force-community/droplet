# Metric 指标解释

本文档简要阐述了每个 metric 的具体含义。

## Graphsync的连接状态

```go
// 正在向本节点传输数据的 peer 数量
GraphsyncReceivingPeersCount              = stats.Int64("graphsync/receiving_peers", "number of peers we are receiving graphsync data from", stats.UnitDimensionless)
// 正在接收本节点数据的 peer 的数量
GraphsyncSendingPeersCount              = stats.Int64("graphsync/sending_peers", "number of peers we are sending graphsync data to", stats.UnitDimensionless)
// 正在接收数据的连接的数量
GraphsyncReceivingActiveCount             = stats.Int64("graphsync/receiving_active", "number of active receiving graphsync transfers", stats.UnitDimensionless)
// 待接收数据的连接的数量
GraphsyncReceivingCountCount              = stats.Int64("graphsync/receiving_pending", "number of pending receiving graphsync transfers", stats.UnitDimensionless)
// 正在发送数据的连接的数量
GraphsyncSendingActiveCount             = stats.Int64("graphsync/sending_active", "number of active sending graphsync transfers", stats.UnitDimensionless)
// 等待发送数据的连接的数量
GraphsyncSendingCountCount              = stats.Int64("graphsync/sending_pending", "number of pending sending graphsync transfers", stats.UnitDimensionless)
```

## 内存使用状况
```go
// 已分配于接收数据的内存块的数量
GraphsyncReceivingTotalMemoryAllocated    = stats.Int64("graphsync/receiving_total_allocated", "amount of block memory allocated for receiving graphsync data", stats.UnitBytes)
// 等待被释放的用于接收数据的内存块的数量
GraphsyncReceivingTotalPendingAllocations = stats.Int64("graphsync/receiving_pending_allocations", "amount of block memory on hold being received pending allocation", stats.UnitBytes)
// 因为可用接收内存不足而被挂起的 peer 的数量
GraphsyncReceivingPeersPending            = stats.Int64("graphsync/receiving_peers_pending", "number of peers we can't receive more data from cause of pending allocations", stats.UnitDimensionless)

// 已分配用于发送数据的内存块的数量
GraphsyncSendingTotalMemoryAllocated    = stats.Int64("graphsync/sending_total_allocated", "amount of block memory allocated for sending graphsync data", stats.UnitBytes)
// 等待被释放的用于发送数据的内存块的数量
GraphsyncSendingTotalPendingAllocations = stats.Int64("graphsync/sending_pending_allocations", "amount of block memory on hold from sending pending allocation", stats.UnitBytes)
// 因为可用发送内存不足而被挂起的 peer 的数量
GraphsyncSendingPeersPending            = stats.Int64("graphsync/sending_peers_pending", "number of peers we can't send more data to cause of pending allocations", stats.UnitDimensionless)
```


## DagStore 的相关状态
```go
// DagStore 中的 Retrieval 订单的个数
DagStorePRInitCount      = stats.Int64("dagstore/pr_init_count", "Retrieval init count", stats.UnitDimensionless)
// DagStore 中的 Retrieval 占用的存储容量
DagStorePRBytesRequested = stats.Int64("dagstore/pr_requested_bytes", "Retrieval requested bytes", stats.UnitBytes)
// DagStore active shard 数量
DagStoreActiveShardCount = stats.Int64("dagstore/active_shard_count", "Active shard count", stats.UnitMilliseconds)

```

## piecestore 的相关状态
```go
// retrieval deal 正好命中 piecestore 中的 piece 的次数
StorageRetrievalHitCount = stats.Int64("piecestorage/retrieval_hit", "PieceStorage hit count for retrieval", stats.UnitDimensionless)
// 保存 piece 时正好命中 piecestore 中的 piece 的次数
StorageSaveHitCount      = stats.Int64("piecestorage/save_hit", "PieceStorage hit count for save piece data", stats.UnitDimensionless)
```

### rpc
```go
# 调用无效RPC方法的次数
RPCInvalidMethod = stats.Int64("rpc/invalid_method", "Total number of invalid RPC methods called", stats.UnitDimensionless)
# RPC请求失败的次数
RPCRequestError  = stats.Int64("rpc/request_error", "Total number of request errors handled", stats.UnitDimensionless)
# RPC响应失败的次数
RPCResponseError = stats.Int64("rpc/response_error", "Total number of responses errors handled", stats.UnitDimensionless)
```

### spark deal index
```go
# Spark 可用的订单数量
SparkEligibleDealCount = stats.Int64("spark_eligible_deal_count", "Spark eligible deal count", stats.UnitDimensionless)
# Spark 检索成功率
SparkRetrievalRate = stats.Int64("spark_retrieval_rate", "Spark retrieval rate", stats.UnitDimensionless)
```
