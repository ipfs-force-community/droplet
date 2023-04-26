package badger

import (
	"context"
	"path"

	"github.com/filecoin-project/venus-market/v2/models/badger/migrate"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/ipfs-force-community/metrics"
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
	storageProvider   = "/storage/provider"
	storageDeals      = "/deals"
	storageAsk        = "/storage-ask"
	paych             = "/paych/"

	// client
	dealClient      = "/deals/client"
	dealLocal       = "/deals/local"
	offlineDeal     = "/deals/offline"
	retrievalClient = "/retrievals/client"
	clientTransfer  = "/datatransfer/client/transfers"
)

// /metadata
type MetadataDS datastore.Batching

// /metadata/fundmgr
type FundMgrDS datastore.Batching

// /metadata/storagemarket
type PieceMetaDs datastore.Batching

// /metadata/storagemarket/cid-infos
type CIDInfoDS datastore.Batching

// /metadata/storagemarket/pieces
type PieceInfoDS datastore.Batching

// /metadata/retrievals/provider
type RetrievalProviderDS datastore.Batching

// /metadata/retrievals/provider/deals
type RetrievalDealsDS datastore.Batching

// /metadata/retrievals/provider/retrieval-ask
type RetrievalAskDS datastore.Batching // key = latest

// /metadata/datatransfer/provider/transfers
type DagTransferDS datastore.Batching

// /metadata/storage/provider
type StorageProviderDS datastore.Batching

// /metadata/storage/provider/deals
type StorageDealsDS datastore.Batching

// /metadata/storage/provider/storage-ask
type StorageAskDS datastore.Batching // key = latest

// /metadata/paych/
type PayChanDS datastore.Batching

// /metadata/paych/ChannelInfo
type PayChanInfoDS datastore.Batching

// /metadata/paych/MsgCid
type PayChanMsgDs datastore.Batching

// *********************************client
// /metadata/deals/client
type ClientDatastore datastore.Batching

// /metadata/deals/local
type ClientDealsDS datastore.Batching

// /metadata/deals/offline
type ClientOfflineDealsDS datastore.Batching

// /metadata/retrievals/client
type RetrievalClientDS datastore.Batching

// /metadata/client
type ImportClientDS datastore.Batching

// /metadata/datatransfer/client/transfers
type ClientTransferDS datastore.Batching

func NewMetadataDS(mctx metrics.MetricsCtx, lc fx.Lifecycle, homeDir *config.HomeDir) (MetadataDS, error) {
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

func NewStorageProviderDS(ds MetadataDS) StorageProviderDS {
	return namespace.Wrap(ds, datastore.NewKey(storageProvider))
}

func NewStorageDealsDS(ds StorageProviderDS) StorageDealsDS {
	return namespace.Wrap(ds, datastore.NewKey(storageDeals))
}

func NewStorageAskDS(ds StorageProviderDS) StorageAskDS {
	return namespace.Wrap(ds, datastore.NewKey(storageAsk))
}

func NewPayChanDS(ds MetadataDS) PayChanDS {
	return namespace.Wrap(ds, datastore.NewKey(paych))
}

func NewPayChanInfoDs(ds PayChanDS) PayChanInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(dsKeyChannelInfo))
}

func NewPayChanMsgDs(ds PayChanDS) PayChanMsgDs {
	return namespace.Wrap(ds, datastore.NewKey(dsKeyMsgCid))
}

// NewClientDatastore creates a datastore for the client to store its deals
func NewClientDatastore(ds MetadataDS) ClientDatastore {
	return namespace.Wrap(ds, datastore.NewKey(dealClient))
}

// NewClientOfflineDealStore creates a datastore for the client to store its offline deals
func NewClientOfflineDealStore(ds MetadataDS) ClientOfflineDealsDS {
	return namespace.Wrap(ds, datastore.NewKey(offlineDeal))
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

// nolint
type BadgerRepo struct {
	dsParams *BadgerDSParams
}

// nolint
type BadgerDSParams struct {
	fx.In
	FundDS           FundMgrDS        `optional:"true"`
	StorageDealsDS   StorageDealsDS   `optional:"true"`
	PaychInfoDS      PayChanInfoDS    `optional:"true"`
	PaychMsgDS       PayChanMsgDs     `optional:"true"`
	AskDS            StorageAskDS     `optional:"true"`
	RetrAskDs        RetrievalAskDS   `optional:"true"`
	CidInfoDs        CIDInfoDS        `optional:"true"`
	RetrievalDealsDs RetrievalDealsDS `optional:"true"`
}

func NewBadgerRepo(params BadgerDSParams) repo.Repo {
	return &BadgerRepo{
		dsParams: &params,
	}
}

func NewMigratedBadgerRepo(params BadgerDSParams) (repo.Repo, error) {
	repo := NewBadgerRepo(params)
	return repo, repo.Migrate()
}

func (r *BadgerRepo) FundRepo() repo.FundRepo {
	return NewFundRepo(r.dsParams.FundDS)
}

func (r *BadgerRepo) StorageDealRepo() repo.StorageDealRepo {
	return NewStorageDealRepo(r.dsParams.StorageDealsDS)
}

func (r *BadgerRepo) PaychMsgInfoRepo() repo.PaychMsgInfoRepo {
	return NewPayMsgRepo(r.dsParams.PaychMsgDS)
}

func (r *BadgerRepo) PaychChannelInfoRepo() repo.PaychChannelInfoRepo {
	return NewPaychRepo(r.dsParams.PaychInfoDS, r.PaychMsgInfoRepo())
}

func (r *BadgerRepo) StorageAskRepo() repo.IStorageAskRepo {
	return NewStorageAskRepo(r.dsParams.AskDS)
}

func (r *BadgerRepo) RetrievalAskRepo() repo.IRetrievalAskRepo {
	return NewRetrievalAskRepo(r.dsParams.RetrAskDs)
}

func (r *BadgerRepo) CidInfoRepo() repo.ICidInfoRepo {
	return NewBadgerCidInfoRepo(r.dsParams.CidInfoDs)
}

func (r *BadgerRepo) RetrievalDealRepo() repo.IRetrievalDealRepo {
	return NewRetrievalDealRepo(r.dsParams.RetrievalDealsDs)
}

func (r *BadgerRepo) Close() error {
	// todo: to implement
	return nil
}

func (r *BadgerRepo) Migrate() error {
	ctx := context.TODO()

	migrateDss := map[string]datastore.Batching{
		migrate.DsNameFundedAddrState:  r.dsParams.FundDS,
		migrate.DsNameStorageDeal:      r.dsParams.StorageDealsDS,
		migrate.DsNamePaychInfoDs:      r.dsParams.PaychInfoDS,
		migrate.DsNamePaychMsgDs:       r.dsParams.PaychMsgDS,
		migrate.DsNameStorageAskDs:     r.dsParams.AskDS,
		migrate.DsNameRetrievalAskDs:   r.dsParams.RetrAskDs,
		migrate.DsNameCidInfoDs:        r.dsParams.CidInfoDs,
		migrate.DsNameRetrievalDealsDs: r.dsParams.RetrievalDealsDs,
	}
	// the returned 'newDss' would be wrapped with current version namespace.
	// so, must set all 'ds' back later.
	newDss, err := migrate.Migrate(ctx, migrateDss)
	if err != nil {
		return err
	}
	r.dsParams.FundDS = newDss[migrate.DsNameFundedAddrState]
	r.dsParams.StorageDealsDS = newDss[migrate.DsNameStorageDeal]
	r.dsParams.PaychMsgDS = newDss[migrate.DsNamePaychMsgDs]
	r.dsParams.PaychInfoDS = newDss[migrate.DsNamePaychInfoDs]
	r.dsParams.AskDS = newDss[migrate.DsNameStorageAskDs]
	r.dsParams.RetrAskDs = newDss[migrate.DsNameRetrievalAskDs]
	r.dsParams.CidInfoDs = newDss[migrate.DsNameCidInfoDs]
	r.dsParams.RetrievalDealsDs = newDss[migrate.DsNameRetrievalDealsDs]
	return nil
}

// Not a real transaction
func (r *BadgerRepo) Transaction(cb func(txRepo repo.TxRepo) error) error {
	return cb(&txRepo{dsParams: r.dsParams})
}

type txRepo struct {
	dsParams *BadgerDSParams
}

func (r txRepo) StorageDealRepo() repo.StorageDealRepo {
	return NewStorageDealRepo(r.dsParams.StorageDealsDS)
}

// not metadata, just raw data between file transfer

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
