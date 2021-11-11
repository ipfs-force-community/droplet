package sealer

import (
	"context"
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
func NewMinerAPI(lc fx.Lifecycle, r *config.DAGStoreConfig, repo repo.Repo, pieceStorage piece.IPieceStorage) (dagstore.MarketAPI, error) {
	mountApi := dagstore.NewMinerAPI(repo, pieceStorage, r.MaxConcurrencyStorageCalls)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return mountApi.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return nil
		},
	})

	return mountApi, nil
}
