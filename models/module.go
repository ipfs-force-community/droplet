package models

import (
	"context"
	"github.com/filecoin-project/venus-market/models/itf"
	"path"

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

func NewMetadataDS(mctx metrics.MetricsCtx, lc fx.Lifecycle, homeDir *config.HomeDir) (itf.MetadataDS, error) {
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

func NewPieceMetaDs(ds itf.MetadataDS) itf.PieceMetaDs {
	return namespace.Wrap(ds, datastore.NewKey(piecemeta))
}

func NewFundMgrDS(ds itf.MetadataDS) itf.FundMgrDS {
	return namespace.Wrap(ds, datastore.NewKey(fundmgr))
}

func NewCidInfoDs(ds itf.PieceMetaDs) itf.CIDInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(cidinfo))
}

func NewPieceInfoDs(ds itf.PieceMetaDs) itf.PieceInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(pieceinfo))
}

func NewRetrievalProviderDS(ds itf.MetadataDS) itf.RetrievalProviderDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalProvider))
}

func NewRetrievalAskDS(ds itf.RetrievalProviderDS) itf.RetrievalAskDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalAsk))
}

func NewDagTransferDS(ds itf.MetadataDS) itf.DagTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(transfer))
}

func NewProviderDealDS(ds itf.MetadataDS) itf.ProviderDealDS {
	return namespace.Wrap(ds, datastore.NewKey(dealProvider))
}

func NewStorageAskDS(ds itf.ProviderDealDS) itf.StorageAskDS {
	return namespace.Wrap(ds, datastore.NewKey(storageAsk))
}

func NewPayChanDS(ds itf.MetadataDS) itf.PayChanDS {
	return namespace.Wrap(ds, datastore.NewKey(paych))
}

// NewClientDatastore creates a datastore for the client to store its deals
func NewClientDatastore(ds itf.MetadataDS) itf.ClientDatastore {
	return namespace.Wrap(ds, datastore.NewKey(dealClient))
}

// for discover
func NewClientDealsDS(ds itf.MetadataDS) itf.ClientDealsDS {
	return namespace.Wrap(ds, datastore.NewKey(dealLocal))
}

func NewRetrievalClientDS(ds itf.MetadataDS) itf.RetrievalClientDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalClient))
}

func NewImportClientDS(ds itf.MetadataDS) itf.ImportClientDS {
	return namespace.Wrap(ds, datastore.NewKey(client))
}

func NewClientTransferDS(ds itf.MetadataDS) itf.ClientTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(clientTransfer))
}

var DBOptions = func(server bool) builder.Option {
	if server {
		return builder.Options(
			builder.Override(new(itf.MetadataDS), NewMetadataDS),
			builder.Override(new(StagingDS), NewStagingDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(itf.PieceMetaDs), NewPieceMetaDs),
			builder.Override(new(itf.PieceInfoDS), NewPieceInfoDs),
			builder.Override(new(itf.CIDInfoDS), NewCidInfoDs),
			builder.Override(new(itf.RetrievalProviderDS), NewRetrievalProviderDS),
			builder.Override(new(itf.RetrievalAskDS), NewRetrievalAskDS),
			builder.Override(new(itf.DagTransferDS), NewDagTransferDS),
			builder.Override(new(itf.ProviderDealDS), NewProviderDealDS),
			builder.Override(new(itf.StorageAskDS), NewStorageAskDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(itf.PayChanDS), NewPayChanDS),
			builder.Override(new(itf.FundMgrDS), NewFundMgrDS),

			// if there is a mysql connection string exist,
			// use mysql storage_ask_ds, otherwise use a badger
			builder.Override(new(itf.Repo), func(cfg *config.Mysql,
				fundDS itf.FundMgrDS, dealDS itf.ProviderDealDS, retrievalDs itf.RetrievalProviderDS,
				paychDS itf.PayChanDS, askDS itf.StorageAskDS) (itf.Repo, error) {
				if len(cfg.ConnectionString) == 0 {
					return badger_models.NewBadgerRepo(fundDS, dealDS, retrievalDs, paychDS, askDS), nil
				}
				return mysql.InitMysql(cfg)
			}),
		)
	} else {
		return builder.Options(
			builder.Override(new(itf.MetadataDS), NewMetadataDS),
			builder.Override(new(itf.FundMgrDS), NewFundMgrDS),
			builder.Override(new(itf.PayChanDS), NewPayChanDS),

			builder.Override(new(itf.ClientDatastore), NewClientDatastore),
			builder.Override(new(ClientBlockstore), NewClientBlockstore),
			builder.Override(new(itf.ClientDealsDS), NewClientDealsDS),
			builder.Override(new(itf.RetrievalClientDS), NewRetrievalClientDS),
			builder.Override(new(itf.ImportClientDS), NewImportClientDS),
			builder.Override(new(itf.ClientTransferDS), NewClientTransferDS),
			builder.Override(new(itf.Repo), func() itf.Repo {
				return nil
			}),
		)
	}
}
