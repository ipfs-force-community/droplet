package models

import (
	"context"
	"path"

	"github.com/filecoin-project/venus-market/models/repo"

	badger_models "github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/mysql"

	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
)

const (
	metadata = "metadata"
	staging  = "staging"
	transfer = "transfers"

	fundmgr           = "/fundmgr/"
	piecemeta         = "/storagemarket"
	cidinfo           = "/cid-infos"
	pieceinfo         = "/pieces"
	retrievalProvider = "/retrievals/provider"
	retrievalAsk      = "/retrieval-ask"
	dealProvider      = "/deals/provider"
	storageAsk        = "storage-ask"
	paych             = "/paych/"

	// client
	client          = "/client"
	dealClient      = "/deals/client"
	dealLocal       = "/deals/local"
	retrievalClient = "/retrievals/client"
	clientTransfer  = "/datatransfer/client/transfers"
)

func NewMetadataDS(mctx metrics.MetricsCtx, lc fx.Lifecycle, homeDir *config.HomeDir) (repo.MetadataDS, error) {
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

func NewPieceMetaDs(ds repo.MetadataDS) repo.PieceMetaDs {
	return namespace.Wrap(ds, datastore.NewKey(piecemeta))
}

func NewFundMgrDS(ds repo.MetadataDS) repo.FundMgrDS {
	return namespace.Wrap(ds, datastore.NewKey(fundmgr))
}

func NewCidInfoDs(ds repo.PieceMetaDs) repo.CIDInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(cidinfo))
}

func NewPieceInfoDs(ds repo.PieceMetaDs) repo.PieceInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(pieceinfo))
}

func NewRetrievalProviderDS(ds repo.MetadataDS) repo.RetrievalProviderDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalProvider))
}

func NewRetrievalAskDS(ds repo.RetrievalProviderDS) repo.RetrievalAskDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalAsk))
}

func NewDagTransferDS(ds repo.MetadataDS) repo.DagTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(transfer))
}

func NewProviderDealDS(ds repo.MetadataDS) repo.ProviderDealDS {
	return namespace.Wrap(ds, datastore.NewKey(dealProvider))
}

func NewStorageAskDS(ds repo.ProviderDealDS) repo.StorageAskDS {
	return namespace.Wrap(ds, datastore.NewKey(storageAsk))
}

func NewPayChanDS(ds repo.MetadataDS) repo.PayChanDS {
	return namespace.Wrap(ds, datastore.NewKey(paych))
}

// NewClientDatastore creates a datastore for the client to store its deals
func NewClientDatastore(ds repo.MetadataDS) repo.ClientDatastore {
	return namespace.Wrap(ds, datastore.NewKey(dealClient))
}

// for discover
func NewClientDealsDS(ds repo.MetadataDS) repo.ClientDealsDS {
	return namespace.Wrap(ds, datastore.NewKey(dealLocal))
}

func NewRetrievalClientDS(ds repo.MetadataDS) repo.RetrievalClientDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalClient))
}

func NewImportClientDS(ds repo.MetadataDS) repo.ImportClientDS {
	return namespace.Wrap(ds, datastore.NewKey(client))
}

func NewClientTransferDS(ds repo.MetadataDS) repo.ClientTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(clientTransfer))
}

var DBOptions = func(server bool, mysqlCfg *config.Mysql) builder.Option {
	if server {
		commonOpts := builder.Options(
			builder.Override(new(repo.MetadataDS), NewMetadataDS),
			builder.Override(new(StagingDS), NewStagingDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(repo.PieceMetaDs), NewPieceMetaDs),
			builder.Override(new(repo.DagTransferDS), NewDagTransferDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
		)
		var opts builder.Option
		if len(mysqlCfg.ConnectionString) > 0 {
			opts = builder.Override(new(repo.Repo), func() (repo.Repo, error) {
				return mysql.InitMysql(mysqlCfg)
			})
		} else {
			opts = builder.Options(
				builder.Override(new(repo.PieceInfoDS), NewPieceInfoDs),
				builder.Override(new(repo.CIDInfoDS), NewCidInfoDs),
				builder.Override(new(repo.RetrievalProviderDS), NewRetrievalProviderDS),
				builder.Override(new(repo.RetrievalAskDS), NewRetrievalAskDS),
				builder.Override(new(repo.ProviderDealDS), NewProviderDealDS),
				builder.Override(new(repo.StorageAskDS), NewStorageAskDS),
				builder.Override(new(repo.PayChanDS), NewPayChanDS),
				builder.Override(new(repo.FundMgrDS), NewFundMgrDS),
				builder.Override(new(repo.Repo), func(fundDS repo.FundMgrDS, dealDS repo.ProviderDealDS,
					paychDS repo.PayChanDS, askDS repo.StorageAskDS, retrAskDs repo.RetrievalAskDS,
					pieceDs repo.PieceInfoDS, cidInfoDs repo.CIDInfoDS, retrievalDs repo.RetrievalProviderDS) (repo.Repo, error) {
					return badger_models.NewBadgerRepo(fundDS, dealDS, paychDS, askDS, retrAskDs, cidInfoDs, retrievalDs)
				}),
			)
		}
		return builder.Options(commonOpts, opts, builder.Override(new(repo.FundRepo), func(repo repo.Repo) repo.FundRepo {
			return repo.FundRepo()
		}))
	} else {
		return builder.Options(
			builder.Override(new(repo.MetadataDS), NewMetadataDS),
			builder.Override(new(repo.FundMgrDS), NewFundMgrDS),
			builder.Override(new(repo.PayChanDS), NewPayChanDS),
			builder.Override(new(repo.ClientDatastore), NewClientDatastore),
			builder.Override(new(ClientBlockstore), NewClientBlockstore),
			builder.Override(new(repo.ClientDealsDS), NewClientDealsDS),
			builder.Override(new(repo.RetrievalClientDS), NewRetrievalClientDS),
			builder.Override(new(repo.ImportClientDS), NewImportClientDS),
			builder.Override(new(repo.ClientTransferDS), NewClientTransferDS),
			builder.Override(new(repo.FundRepo), func(fundDS repo.FundMgrDS) repo.FundRepo {
				return badger_models.NewFundRepo(fundDS)
			}),
		)
	}
}
