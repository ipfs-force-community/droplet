package utils

import (
	"os"
	"os/signal"
	"syscall"
)

func MonitorShutdown(triggerCh <-chan struct{}) <-chan struct{} {
	sigCh := make(chan os.Signal, 2)
	out := make(chan struct{})

	go func() {
		select {
		case sig := <-sigCh:
			log.Warnw("received shutdown", "signal", sig)
		case <-triggerCh:
			log.Warn("received shutdown")
		}

		log.Warn("Shutting down...")

		log.Warn("Graceful shutdown successful")

		// Sync all loggers.
		_ = log.Sync() //nolint:errcheck
		close(out)
	}()

	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	return out
}
