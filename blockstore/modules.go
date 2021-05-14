package blockstore

import (
	"context"
	dgbadger "github.com/dgraph-io/badger/v2"
	"github.com/filecoin-project/go-multistore"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/ipfs/go-datastore"

	badger "github.com/ipfs/go-ds-badger2"
	"go.uber.org/fx"
)

func OpenMetadataDs(lc fx.Lifecycle, cfg *config.TransferConfig) (datastore.Batching, error) {
	bs, err := badgerDs(cfg.MetaDs+"/metadata", false)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return bs.Close()
		},
	})

	return bs, nil
}

func ClientMultiDatastore(lc fx.Lifecycle, cfg *config.TransferConfig) (ClientMultiDstore, error) {
	bs, err := badgerDs(cfg.Path+"/client", false)
	if err != nil {
		return nil, err
	}

	mds, err := multistore.NewMultiDstore(bs)
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

//server

func StaagingDs_(cfg *config.TransferConfig) (StagingDs, error) {
	return badgerDs(cfg.Path+"/staging", false)
}

func StagingDatastore(lc fx.Lifecycle, stagingds StagingDs) (ClientMultiDstore, error) {
	mds, err := multistore.NewMultiDstore(stagingds)
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

// StagingBlockstore creates a blockstore for staging blocks for a miner
// in a storage deal, prior to sealing
func StagingBlockstore_(lc fx.Lifecycle, stagingds StagingDs) (StagingBlockstore, error) {
	return FromDatastore(stagingds), nil
}

func badgerDs(path string, readonly bool) (datastore.Batching, error) {
	opts := badger.DefaultOptions
	opts.ReadOnly = readonly

	opts.Options = dgbadger.DefaultOptions("").WithTruncate(true).
		WithValueThreshold(1 << 10)
	return badger.NewDatastore(path, &opts)
}
