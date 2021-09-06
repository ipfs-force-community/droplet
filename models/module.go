package models

import (
	"context"
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
	"path"
)

const (
	metadata = "metadata"
	staging  = "staging"

	piecemeta         = "/storagemarket"
	retrievalProvider = "/retrievals/provider"
	retrievalAsk      = "/retrieval-ask"
	transfer          = "transfers"
	dealProvider      = "/deals/provider"
	storageAsk        = "storage-ask"
	paych             = "/paych/"
	dealClient        = "/deals/client"
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

func NewPayChanDS(ds MetadataDS) StorageAskDS {
	return namespace.Wrap(ds, datastore.NewKey(paych))
}

// NewClientDatastore creates a datastore for the client to store its deals
func NewClientDatastore(ds MetadataDS) ClientDatastore {
	return namespace.Wrap(ds, datastore.NewKey(dealClient))
}

var DBOptions = func(server bool) builder.Option {
	if server {
		return builder.Options(
			builder.Override(new(MetadataDS), NewMetadataDS),
			builder.Override(new(StagingDS), NewStagingDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(PieceMetaDs), NewPieceMetaDs),
			builder.Override(new(RetrievalProviderDS), NewRetrievalProviderDS),
			builder.Override(new(RetrievalAskDS), NewRetrievalAskDS),
			builder.Override(new(DagTransferDS), NewDagTransferDS),
			builder.Override(new(ProviderDealDS), NewProviderDealDS),
			builder.Override(new(StorageAskDS), NewStorageAskDS),
			builder.Override(new(StagingBlockstore), NewStagingBlockStore),
			builder.Override(new(PayChanDS), NewPayChanDS),
		)
	} else {
		return builder.Options(
			builder.Override(new(MetadataDS), NewMetadataDS),
			builder.Override(new(ClientDatastore), NewClientDatastore),
			builder.Override(new(ClientBlockstore), NewClientBlockstore),
			builder.Override(new(PayChanDS), NewPayChanDS),
		)
	}
}
