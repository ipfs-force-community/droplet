package repo

import (
	"github.com/ipfs/go-datastore"
)

// /metadata
type MetadataDS datastore.Batching

// /metadata/fundmgr
type FundMgrDS datastore.Batching

// /metadata/storagemarket
type PieceMetaDs datastore.Batching

//  /metadata/storagemarket/cid-infos
type CIDInfoDS datastore.Batching

//  /metadata/storagemarket/pieces
type PieceInfoDS datastore.Batching

// /metadata/retrievals/provider
type RetrievalProviderDS datastore.Batching

// /metadata/retrievals/provider/retrieval-ask
type RetrievalAskDS datastore.Batching //key = latest

// /metadata/datatransfer/provider/transfers
type DagTransferDS datastore.Batching

// /metadata/deals/provider
type ProviderDealDS datastore.Batching

//   /metadata/deals/provider/storage-ask
type StorageAskDS datastore.Batching //key = latest

// /metadata/paych/
type PayChanDS datastore.Batching

//*********************************client
// /metadata/deals/client
type ClientDatastore datastore.Batching

// /metadata/deals/local
type ClientDealsDS datastore.Batching

// /metadata/retrievals/client
type RetrievalClientDS datastore.Batching

// /metadata/client
type ImportClientDS datastore.Batching

// /metadata/datatransfer/client/transfers
type ClientTransferDS datastore.Batching
