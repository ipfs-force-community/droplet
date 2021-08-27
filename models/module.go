package models

import (
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
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
)

func NewMetadataDS(cfg *config.MarketConfig) (MetadataDS, error) {
	metaDataPath, err := cfg.HomeJoin(metadata)
	if err != nil {
		return nil, err
	}
	return badger.NewDatastore(metaDataPath, &badger.DefaultOptions)
}

func NewStagingDS(cfg *config.MarketConfig) (StagingDS, error) {
	metaDataPath, err := cfg.HomeJoin(staging)
	if err != nil {
		return nil, err
	}
	return badger.NewDatastore(metaDataPath, &badger.DefaultOptions)
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

var DBOptions = builder.Options(
	builder.Override(new(MetadataDS), NewMetadataDS),
	builder.Override(new(StagingDS), NewStagingDS),
	builder.Override(new(StagingBlockstore), NewStagingBlockStore),
	builder.Override(new(PieceMetaDs), NewPieceMetaDs),
	builder.Override(new(RetrievalProviderDS), NewRetrievalProviderDS),
	builder.Override(new(RetrievalAskDS), NewRetrievalAskDS),
	builder.Override(new(DagTransferDS), NewDagTransferDS),
	builder.Override(new(ProviderDealDS), NewProviderDealDS),
	builder.Override(new(StorageAskDS), NewStorageAskDS),
	builder.Override(new(PayChanDS), NewPayChanDS),
)
