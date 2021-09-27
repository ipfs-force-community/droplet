package sealer

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus-market/dagstore"
	"go.uber.org/fx"

	"github.com/filecoin-project/venus-market/config"
)

const (
	EnvDAGStoreCopyConcurrency = "LOTUS_DAGSTORE_COPY_CONCURRENCY"
	DefaultDAGStoreDir         = "dagstore"
)

// NewMinerAPI creates a new MinerAPI adaptor for the dagstore mounts.
func NewMinerAPI(lc fx.Lifecycle, r *config.DAGStoreConfig, pieceStore piecestore.PieceStore, sa retrievalmarket.SectorAccessor) (dagstore.MinerAPI, error) {
	mountApi := dagstore.NewMinerAPI(pieceStore, sa, r.MaxConcurrencyStorageCalls)
	ready := make(chan error, 1)
	pieceStore.OnReady(func(err error) {
		ready <- err
	})
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := <-ready; err != nil {
				return fmt.Errorf("aborting dagstore start; piecestore failed to start: %s", err)
			}
			return mountApi.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return nil
		},
	})

	return mountApi, nil
}
