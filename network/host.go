package network

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/version"
	"github.com/ipfs-force-community/metrics"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"go.uber.org/fx"
)

type P2PHostIn struct {
	fx.In

	ID        peer.ID
	Peerstore peerstore.Peerstore

	Opts [][]libp2p.Option `group:"libp2p"`
}

// ////////////////////////
func Host(mctx metrics.MetricsCtx, lc fx.Lifecycle, params P2PHostIn, cfg *config.Libp2p) (host.Host, error) {
	pkey := params.Peerstore.PrivKey(params.ID)
	if pkey == nil {
		return nil, fmt.Errorf("missing private key for node ID: %s", params.ID.String())
	}

	cm, err := connmgr.NewConnManager(100, 200, connmgr.WithGracePeriod(2*time.Minute))
	if err != nil {
		return nil, err
	}

	addrsFactory, err := makeAddrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.Identity(pkey),
		libp2p.Peerstore(params.Peerstore),
		libp2p.NoListenAddrs,
		libp2p.Ping(true),
		libp2p.ConnectionManager(cm),
		libp2p.AddrsFactory(addrsFactory),
		libp2p.UserAgent("droplet" + version.UserVersion()),
	}
	for _, o := range params.Opts {
		opts = append(opts, o...)
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return h.Close()
		},
	})

	return h, nil
}
