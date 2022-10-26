package network

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"

	"github.com/filecoin-project/venus-market/v2/config"

	"go.uber.org/fx"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
)

func ResourceManager(lc fx.Lifecycle, homeDir *config.HomeDir) (network.ResourceManager, error) {
	repoPath := string(*homeDir)
	// Adjust default defaultLimits
	// - give it more memory, up to 4G, min of 1G
	// - if maxconns are too high, adjust Conn/FD/Stream defaultLimits
	defaultLimits := rcmgr.DefaultLimits

	// TODO: also set appropriate default limits for lotus protocols
	libp2p.SetDefaultServiceLimits(&defaultLimits)

	// Minimum 1GB of memory
	defaultLimits.SystemBaseLimit.Memory = 1 << 30
	// For every extra 1GB of memory we have available, increase our limit by 1GiB
	defaultLimits.SystemLimitIncrease.Memory = 1 << 30
	defaultLimitConfig := defaultLimits.AutoScale()
	if defaultLimitConfig.System.Memory > 4<<30 {
		// Cap our memory limit
		defaultLimitConfig.System.Memory = 4 << 30
	}

	maxconns := int(200) // make config
	if 2*maxconns > defaultLimitConfig.System.ConnsInbound {
		// adjust conns to 2x to allow for two conns per peer (TCP+QUIC)
		defaultLimitConfig.System.ConnsInbound = logScale(2 * maxconns)
		defaultLimitConfig.System.ConnsOutbound = logScale(2 * maxconns)
		defaultLimitConfig.System.Conns = logScale(4 * maxconns)

		defaultLimitConfig.System.StreamsInbound = logScale(16 * maxconns)
		defaultLimitConfig.System.StreamsOutbound = logScale(64 * maxconns)
		defaultLimitConfig.System.Streams = logScale(64 * maxconns)

		if 2*maxconns > defaultLimitConfig.System.FD {
			defaultLimitConfig.System.FD = logScale(2 * maxconns)
		}

		defaultLimitConfig.ServiceDefault.StreamsInbound = logScale(8 * maxconns)
		defaultLimitConfig.ServiceDefault.StreamsOutbound = logScale(32 * maxconns)
		defaultLimitConfig.ServiceDefault.Streams = logScale(32 * maxconns)

		defaultLimitConfig.ProtocolDefault.StreamsInbound = logScale(8 * maxconns)
		defaultLimitConfig.ProtocolDefault.StreamsOutbound = logScale(32 * maxconns)
		defaultLimitConfig.ProtocolDefault.Streams = logScale(32 * maxconns)

		log.Info("adjusted default resource manager limits")
	}

	// initialize
	var limiter rcmgr.Limiter
	var opts []rcmgr.Option

	// create limiter -- parse $repo/limits.json if exists
	limitsFile := filepath.Join(repoPath, "limits.json")
	limitsIn, err := os.Open(limitsFile)
	switch {
	case err == nil:
		defer limitsIn.Close() //nolint:errcheck
		limiter, err = rcmgr.NewLimiterFromJSON(limitsIn, defaultLimitConfig)
		if err != nil {
			return nil, fmt.Errorf("error parsing limit file: %w", err)
		}

	case errors.Is(err, os.ErrNotExist):
		limiter = rcmgr.NewFixedLimiter(defaultLimitConfig)

	default:
		return nil, err
	}

	if os.Getenv("MARKET_DEBUG_RCMGR") != "" {
		debugPath := filepath.Join(repoPath, "debug")
		if err := os.MkdirAll(debugPath, 0o755); err != nil {
			return nil, fmt.Errorf("error creating debug directory: %w", err)
		}
		traceFile := filepath.Join(debugPath, "rcmgr.json.gz")
		opts = append(opts, rcmgr.WithTrace(traceFile))
	}

	mgr, err := rcmgr.NewResourceManager(limiter, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating resource manager: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return mgr.Close()
		},
	})

	return mgr, nil
}

func ResourceManagerOption(mgr network.ResourceManager) Libp2pOpts {
	return Libp2pOpts{
		Opts: []libp2p.Option{libp2p.ResourceManager(mgr)},
	}
}

func logScale(val int) int {
	bitlen := bits.Len(uint(val))
	return 1 << bitlen
}
