package sealer

import (
	"context"
	"fmt"
	"github.com/filecoin-project/venus-market/dagstore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/piece"
	"go.uber.org/fx"

	"github.com/filecoin-project/venus-market/config"
)

const (
	EnvDAGStoreCopyConcurrency = "LOTUS_DAGSTORE_COPY_CONCURRENCY"
	DefaultDAGStoreDir         = "dagstore"
)

// NewMinerAPI creates a new MarketAPI adaptor for the dagstore mounts.
func NewMinerAPI(lc fx.Lifecycle, r *config.DAGStoreConfig, pieceRepo repo.IPieceRepo, pieceStorage piece.PieceStorage) (dagstore.MarketAPI, error) {
	mountApi := dagstore.NewMinerAPI(pieceRepo, pieceStorage, r.MaxConcurrencyStorageCalls)
	ready := make(chan error, 1)
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
