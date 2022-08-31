package metrics

import (
	"context"

	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	metrics2 "github.com/ipfs/go-metrics-interface"
	"go.uber.org/fx"
)

var startMetricsKey = builder.NextInvoke()

var MetricsOpts = func(scope string, metricsConfig *metrics.MetricsConfig) builder.Option {
	return builder.Options(
		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), scope)
		}),
		builder.Override(startMetricsKey, func(mctx metrics.MetricsCtx, lc fx.Lifecycle) error {
			return SetupMetrics(metrics.LifecycleCtx(mctx, lc), metricsConfig)
		}),
	)
}
