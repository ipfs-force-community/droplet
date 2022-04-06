package dagstore

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/go-fil-markets/stores"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
)

var (
	DAGStoreKey = builder.Special{ID: 1}
)

const (
	EnvDAGStoreCopyConcurrency = "LOTUS_DAGSTORE_COPY_CONCURRENCY"
	DefaultDAGStoreDir         = "dagstore"
)

// NewMarketAPI creates a new MarketAPI adaptor for the dagstore mounts.
func CreateAndStartMarketAPI(lc fx.Lifecycle, r *config.DAGStoreConfig, repo repo.Repo, pieceStorage *piecestorage.PieceStorageManager) (MarketAPI, error) {
	mountApi := NewMarketAPI(repo, pieceStorage, r.UseTransient)
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
func NewWrapperDAGStore(lc fx.Lifecycle, ctx metrics.MetricsCtx, homeDir *config.HomeDir, cfg *config.DAGStoreConfig, minerAPI MarketAPI) (*dagstore.DAGStore, stores.DAGStoreWrapper, error) {
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

	dagst, w, err := NewDAGStore(ctx, cfg, minerAPI)
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
	builder.Override(new(MarketAPI), CreateAndStartMarketAPI),
	builder.Override(DAGStoreKey, NewWrapperDAGStore),
)
