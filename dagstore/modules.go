package dagstore

import (
	"context"
	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	xerrors "github.com/pkg/errors"
	"go.uber.org/fx"
	"os"
	"path/filepath"
	"strconv"
)

var (
	DAGStoreKey = builder.Special{ID: 1}
)

const (
	EnvDAGStoreCopyConcurrency = "LOTUS_DAGSTORE_COPY_CONCURRENCY"
	DefaultDAGStoreDir         = "dagstore"
)

// NewMinerAPI creates a new MarketAPI adaptor for the dagstore mounts.
func NewMarketAPI(lc fx.Lifecycle, r *config.DAGStoreConfig, repo repo.Repo, pieceStorage piecestorage.IPieceStorage) (MarketAPI, error) {
	mountApi := NewMinerAPI(repo, pieceStorage, r.MaxConcurrencyStorageCalls)
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

// DAGStore constructs a DAG store using the supplied minerAPI, and the
// user configuration. It returns both the DAGStore and the Wrapper suitable for
// passing to markets.
func NewWrapperDAGStore(lc fx.Lifecycle, homeDir *config.HomeDir, cfg *config.DAGStoreConfig, minerAPI MarketAPI) (*dagstore.DAGStore, stores.DAGStoreWrapper, error) {
	// fall back to default root directory if not explicitly set in the config.
	if cfg.RootDir == "" {
		cfg.RootDir = filepath.Join(string(*homeDir), DefaultDAGStoreDir)
	}

	v, ok := os.LookupEnv(EnvDAGStoreCopyConcurrency)
	if ok {
		concurrency, err := strconv.Atoi(v)
		if err == nil {
			cfg.MaxConcurrentReadyFetches = concurrency
		}
	}

	dagst, w, err := NewDAGStore(cfg, minerAPI)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to create DAG store: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return w.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return w.Close()
		},
	})

	return dagst, w, nil
}

var DagstoreOpts = builder.Options(
	builder.Override(new(MarketAPI), NewMarketAPI),
	builder.Override(DAGStoreKey, NewWrapperDAGStore),
)
