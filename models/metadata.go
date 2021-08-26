package models

import "github.com/ipfs/go-datastore"

// /metadata
type MetadataDS datastore.Batching

// /storagemarket
type PieceMetaDs datastore.Batching

// /retrievals/provider
type RetrievalProviderDS datastore.Batching

// //retrievals/provider/retrieval-ask
type RetrievalAskDS datastore.Batching //key = latest

// /datatransfer/provider/transfers
type DagTransferDS datastore.Batching

// /deals/provider
type ProviderDealDS datastore.Batching

//   /deals/provider/storage-ask
type StorageAskDS datastore.Batching //key = latest
