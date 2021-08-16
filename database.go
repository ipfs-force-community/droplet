package main

import (
	"context"
	"github.com/filecoin-project/go-multistore"
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/dtypes"
	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
)

func MetadataDs(cfg *config.Market) (dtypes.MetadataDS, error) {
	metaDataPath, err := cfg.HomeJoin("metadata")
	if err != nil {
		return nil, err
	}
	return badger.NewDatastore(metaDataPath, &badger.DefaultOptions)
}

func StageingDs(cfg *config.Market) (dtypes.MetadataDS, error) {
	metaDataPath, err := cfg.HomeJoin("staging")
	if err != nil {
		return nil, err
	}
	return badger.NewDatastore(metaDataPath, &badger.DefaultOptions)
}

func StagingMultiDatastore(lc fx.Lifecycle, stagingDs dtypes.StagingDS) (dtypes.StagingMultiDstore, error) {
	mds, err := multistore.NewMultiDstore(stagingDs)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return mds.Close()
		},
	})

	return mds, nil
}

func StagingBlockStore(lc fx.Lifecycle, stagingDs dtypes.StagingDS) (dtypes.StagingBlockstore, error) {
	return blockstore.FromDatastore(stagingDs), nil
}
