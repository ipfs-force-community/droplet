package badger

import (
	"context"
	"path"

	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
)

const (
	metadata = "metadata"

	fundmgr           = "/fundmgr/"
	piecemeta         = "/storagemarket"
	cidinfo           = "/cid-infos"
	retrievalProvider = "/retrievals/provider"
	retrievalAsk      = "/retrieval-ask"
	retrievalDeals    = "/deals"
	dealProvider      = "/deals/provider"
	storageAsk        = "storage-ask"
	paych             = "/paych/"

	// client
	dealClient      = "/deals/client"
	dealLocal       = "/deals/local"
	retrievalClient = "/retrievals/client"
	clientTransfer  = "/datatransfer/client/transfers"
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

// /metadata/retrievals/provider/deals
type RetrievalDealsDS datastore.Batching

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

func NewMetadataDS(mctx metrics.MetricsCtx, lc fx.Lifecycle, homeDir *config.HomeDir) (MetadataDS, error) {
	datastore.ErrNotFound = repo.ErrNotFound
	db, err := badger.NewDatastore(path.Join(string(*homeDir), metadata), &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

func NewPieceMetaDs(ds MetadataDS) PieceMetaDs {
	return namespace.Wrap(ds, datastore.NewKey(piecemeta))
}

func NewFundMgrDS(ds MetadataDS) FundMgrDS {
	return namespace.Wrap(ds, datastore.NewKey(fundmgr))
}

func NewCidInfoDs(ds PieceMetaDs) CIDInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(cidinfo))
}

func NewRetrievalProviderDS(ds MetadataDS) RetrievalProviderDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalProvider))
}

func NewRetrievalDealsDS(ds RetrievalProviderDS) RetrievalDealsDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalDeals))
}

func NewRetrievalAskDS(ds RetrievalProviderDS) RetrievalAskDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalAsk))
}

func NewProviderDealDS(ds MetadataDS) ProviderDealDS {
	return namespace.Wrap(ds, datastore.NewKey(dealProvider))
}

func NewStorageAskDS(ds ProviderDealDS) StorageAskDS {
	return namespace.Wrap(ds, datastore.NewKey(storageAsk))
}

func NewPayChanDS(ds MetadataDS) PayChanDS {
	return namespace.Wrap(ds, datastore.NewKey(paych))
}

// NewClientDatastore creates a datastore for the client to store its deals
func NewClientDatastore(ds MetadataDS) ClientDatastore {
	return namespace.Wrap(ds, datastore.NewKey(dealClient))
}

// for discover
func NewClientDealsDS(ds MetadataDS) ClientDealsDS {
	return namespace.Wrap(ds, datastore.NewKey(dealLocal))
}

func NewRetrievalClientDS(ds MetadataDS) RetrievalClientDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalClient))
}

func NewClientTransferDS(ds MetadataDS) ClientTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(clientTransfer))
}

type BadgerRepo struct {
	fundRepo         repo.FundRepo
	storageDealRepo  repo.StorageDealRepo
	channelInfoRepo  repo.PaychChannelInfoRepo
	msgInfoRepo      repo.PaychMsgInfoRepo
	storageAskRepo   repo.IStorageAskRepo
	retrievalAskRepo repo.IRetrievalAskRepo
	piecesRepo       repo.ICidInfoRepo
	retrievalRepo    repo.IRetrievalDealRepo
}

type BadgerDSParams struct {
	fx.In
	FundDS           FundMgrDS        `optional:"true"`
	DealDS           ProviderDealDS   `optional:"true"`
	PaychDS          PayChanDS        `optional:"true"`
	AskDS            StorageAskDS     `optional:"true"`
	RetrAskDs        RetrievalAskDS   `optional:"true"`
	CidInfoDs        CIDInfoDS        `optional:"true"`
	RetrievalDealsDs RetrievalDealsDS `optional:"true"`
}

func NewBadgerRepo(params BadgerDSParams) (repo.Repo, error) {
	pst := NewPaychRepo(params.PaychDS)
	return &BadgerRepo{
		fundRepo:         NewFundRepo(params.FundDS),
		storageDealRepo:  NewStorageDealRepo(params.DealDS),
		msgInfoRepo:      pst,
		channelInfoRepo:  pst,
		storageAskRepo:   NewStorageAskRepo(params.AskDS),
		retrievalAskRepo: NewRetrievalAskRepo(params.RetrAskDs),
		piecesRepo:       NewBadgerCidInfoRepo(params.CidInfoDs),
		retrievalRepo:    NewRetrievalDealRepo(params.RetrievalDealsDs),
	}, nil
}

func (r *BadgerRepo) FundRepo() repo.FundRepo {
	return r.fundRepo
}

func (r *BadgerRepo) StorageDealRepo() repo.StorageDealRepo {
	return r.storageDealRepo
}

func (r *BadgerRepo) PaychMsgInfoRepo() repo.PaychMsgInfoRepo {
	return r.msgInfoRepo
}

func (r *BadgerRepo) PaychChannelInfoRepo() repo.PaychChannelInfoRepo {
	return r.channelInfoRepo
}

func (r *BadgerRepo) StorageAskRepo() repo.IStorageAskRepo {
	return r.storageAskRepo
}

func (r *BadgerRepo) RetrievalAskRepo() repo.IRetrievalAskRepo {
	return r.retrievalAskRepo
}

func (r *BadgerRepo) CidInfoRepo() repo.ICidInfoRepo {
	return r.piecesRepo
}

func (r *BadgerRepo) RetrievalDealRepo() repo.IRetrievalDealRepo {
	return r.retrievalRepo
}

func (r *BadgerRepo) Close() error {
	// todo: to implement
	return nil
}

//not metadata, just raw data between file transfer

const (
	staging  = "staging"
	transfer = "transfers"

	// client
	client = "/client"
)

func NewDagTransferDS(ds MetadataDS) DagTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(transfer))
}

func NewStagingDS(mctx metrics.MetricsCtx, lc fx.Lifecycle, homeDir *config.HomeDir) (StagingDS, error) {
	db, err := badger.NewDatastore(path.Join(string(*homeDir), staging), &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})
	return db, nil
}
func NewStagingBlockStore(lc fx.Lifecycle, stagingDs StagingDS) (StagingBlockstore, error) {
	return blockstore.FromDatastore(stagingDs), nil
}

func NewImportClientDS(ds MetadataDS) ImportClientDS {
	return namespace.Wrap(ds, datastore.NewKey(client))
}
