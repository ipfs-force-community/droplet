package models

import (
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/utils"
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

var DBOptions = utils.Options(
	utils.Override(new(MetadataDS), NewMetadataDS),
	utils.Override(new(StagingDS), NewStagingDS),
	utils.Override(new(StagingBlockstore), NewStagingBlockStore),
	utils.Override(new(PieceMetaDs), NewPieceMetaDs),
	utils.Override(new(RetrievalProviderDS), NewRetrievalProviderDS),
	utils.Override(new(RetrievalAskDS), NewRetrievalAskDS),
	utils.Override(new(DagTransferDS), NewDagTransferDS),
	utils.Override(new(ProviderDealDS), NewProviderDealDS),
	utils.Override(new(StorageAskDS), NewStorageAskDS),
)
