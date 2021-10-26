package models

import (
	"context"
	"github.com/filecoin-project/venus-market/models/interfaces"
	"path"

	"github.com/filecoin-project/venus-market/models/mysql"

	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models/StorageAsk"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
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

func NewPieceMetaDs(ds MetadataDS) PieceMetaDs {
	return namespace.Wrap(ds, datastore.NewKey(piecemeta))
}

func NewFundMgrDS(ds MetadataDS) FundMgrDS {
	return namespace.Wrap(ds, datastore.NewKey(fundmgr))
}

func NewCidInfoDs(ds PieceMetaDs) CIDInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(cidinfo))
}

func NewPieceInfoDs(ds PieceMetaDs) PieceInfoDS {
	return namespace.Wrap(ds, datastore.NewKey(pieceinfo))
}

func NewRetrievalProviderDS(ds MetadataDS) RetrievalProviderDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalProvider))
}

func NewRetrievalAskDS(ds RetrievalProviderDS) RetrievalAskDS {
	return namespace.Wrap(ds, datastore.NewKey(retrievalAsk))
}

func NewDagTransferDS(ds MetadataDS) DagTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(transfer))
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

func NewImportClientDS(ds MetadataDS) ImportClientDS {
	return namespace.Wrap(ds, datastore.NewKey(client))
}

func NewClientTransferDS(ds MetadataDS) ClientTransferDS {
	return namespace.Wrap(ds, datastore.NewKey(clientTransfer))
}

var DBOptions = func(server bool) builder.Option {
	if server {
		return builder.Options(
			builder.Override(new(MetadataDS), NewMetadataDS),
			builder.Override(new(StagingDS), NewStagingDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(PieceMetaDs), NewPieceMetaDs),
			builder.Override(new(PieceInfoDS), NewPieceInfoDs),
			builder.Override(new(CIDInfoDS), NewCidInfoDs),
			builder.Override(new(RetrievalProviderDS), NewRetrievalProviderDS),
			builder.Override(new(RetrievalAskDS), NewRetrievalAskDS),
			builder.Override(new(DagTransferDS), NewDagTransferDS),
			builder.Override(new(ProviderDealDS), NewProviderDealDS),
			builder.Override(new(StorageAskDS), NewStorageAskDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(PayChanDS), NewPayChanDS),
			builder.Override(new(FundMgrDS), NewFundMgrDS),

			builder.Override(new(StorageAsk.StorageAskRepo), StorageAsk.NewStorageAsk),
			builder.Override(new(interfaces.Repo), func(cfg *config.Mysql) (interfaces.Repo, error) {
				if len(cfg.ConnectionString) == 0 {
					return nil, xerrors.Errorf("implement me")
				} else {
					return mysql.InitMysql(cfg)
				}
			}),
		)
	} else {
		return builder.Options(
			builder.Override(new(MetadataDS), NewMetadataDS),
			builder.Override(new(FundMgrDS), NewFundMgrDS),
			builder.Override(new(PayChanDS), NewPayChanDS),

			builder.Override(new(ClientDatastore), NewClientDatastore),
			builder.Override(new(ClientBlockstore), NewClientBlockstore),
			builder.Override(new(ClientDealsDS), NewClientDealsDS),
			builder.Override(new(RetrievalClientDS), NewRetrievalClientDS),
			builder.Override(new(ImportClientDS), NewImportClientDS),
			builder.Override(new(ClientTransferDS), NewClientTransferDS),
			builder.Override(new(interfaces.Repo), func() interfaces.Repo {
				return nil
			}),
		)
	}
}
