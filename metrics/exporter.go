package metrics

import (
	"context"
	"fmt"

	"github.com/ipfs-force-community/metrics"
	logging "github.com/ipfs/go-log/v2"
	"go.opencensus.io/stats/view"
)

var log = logging.Logger("metrics")

func SetupMetrics(ctx context.Context, metricsConfig *metrics.MetricsConfig) error {
	log.Infof("metrics config: enabled: %v, exporter type: %s, prometheus: %+v, graphite: %+v",
		metricsConfig.Enabled, metricsConfig.Exporter.Type, metricsConfig.Exporter.Prometheus,
		metricsConfig.Exporter.Graphite)

	if !metricsConfig.Enabled {
		return nil
	}

	if err := view.Register(views...); err != nil {
		return fmt.Errorf("cannot register the view: %w", err)
	}

	switch metricsConfig.Exporter.Type {
	case metrics.ETPrometheus:
		go func() {
			if err := metrics.RegisterPrometheusExporter(ctx, metricsConfig.Exporter.Prometheus); err != nil {
				log.Errorf("failed to register prometheus exporter err: %v", err)
				return
			}
			log.Infof("prometheus exporter server graceful shutdown successful")
		}()

	case metrics.ETGraphite:
		if err := metrics.RegisterGraphiteExporter(ctx, metricsConfig.Exporter.Graphite); err != nil {
			log.Errorf("failed to register graphite exporter: %v", err)
		}
	default:
		log.Warnf("invalid exporter type: %s", metricsConfig.Exporter.Type)
	}
	return nil
}
