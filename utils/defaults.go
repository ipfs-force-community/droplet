package utils

import (
	"context"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/journal"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/types"
	metricsi "github.com/ipfs/go-metrics-interface"
	"go.uber.org/fx"
)

// Basic lotus-app services
func defaults() []Option {
	return []Option{
		// global system journal.
		Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		Override(new(journal.Journal), OpenFilesystemJournal),

		Override(new(metrics.MetricsCtx), func() context.Context {
			return metricsi.CtxScope(context.Background(), "venus-market")
		}),

		Override(new(types.ShutdownChan), make(chan struct{})),
	}
}

func OpenFilesystemJournal(lr *config.MarketConfig, lc fx.Lifecycle, disabled journal.DisabledEvents) (journal.Journal, error) {
	jrnl, err := journal.OpenFSJournal(lr.Journal.Path, disabled)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error { return jrnl.Close() },
	})

	return jrnl, err
}
