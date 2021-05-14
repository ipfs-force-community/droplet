package network

import (
	"context"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	graphsync "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"go.uber.org/fx"
)

type BaseIpfsRouting routing.Routing

type P2PHostIn struct {
	fx.In

	Key crypto.PrivKey

	Opts [][]libp2p.Option `group:"libp2p"`
}

type RawHost host.Host

func Host(mctx metrics.MetricsCtx, lc fx.Lifecycle, params P2PHostIn) (RawHost, error) {
	ctx := metrics.LifecycleCtx(mctx, lc)

	opts := []libp2p.Option{
		libp2p.Identity(params.Key),
		libp2p.NoListenAddrs,
		libp2p.Ping(true),
		libp2p.UserAgent("venus-market" ), //todo add version
	}
	for _, o := range params.Opts {
		opts = append(opts, o...)
	}

	h, err := libp2p.New(ctx, opts...)
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

//todo using ipfs for ?
func RouttodoedHost(rh RawHost, r BaseIpfsRouting) host.Host {
	return routedhost.Wrap(rh, r)
}

func IpfsRouter(mctx metrics.MetricsCtx, lc fx.Lifecycle, host RawHost, dstore blockstore.MetadataDS) (BaseIpfsRouting, error) {
	ctx := metrics.LifecycleCtx(mctx, lc)

	validator := record.NamespacedValidator{
		"pk": record.PublicKeyValidator{},
	}

	opts := []dht.Option{dht.Mode(dht.ModeAuto),
		dht.Datastore(dstore),
		dht.Validator(validator),
	//	dht.ProtocolPrefix(build.DhtProtocolName(nn)),
		dht.QueryFilter(dht.PublicQueryFilter),
		dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
		dht.DisableProviders(),
		dht.DisableValues()}
	d, err := dht.New(
		ctx, host, opts...,
	)

	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return d.Close()
		},
	})

	return d, nil
}

// StagingDAG is a DAGService for the StagingBlockstore
func StagingDAG_(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs blockstore.StagingBlockstore, rt routing.Routing, h host.Host) (StagingDAG, error) {

	bitswapNetwork := network.NewFromIpfsHost(h, rt)
	bitswapOptions := []bitswap.Option{bitswap.ProvideEnabled(false)}
	exch := bitswap.New(mctx, bitswapNetwork, ibs, bitswapOptions...)

	bsvc := blockservice.New(ibs, exch)
	dag := merkledag.NewDAGService(bsvc)

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			// blockservice closes the exchange
			return bsvc.Close()
		},
	})

	return dag, nil
}

// StagingGraphsync creates a graphsync instance which reads and writes blocks
// to the StagingBlockstore
func StagingGraphsync_(mctx metrics.MetricsCtx, lc fx.Lifecycle, ibs blockstore.StagingBlockstore, h host.Host) dtypes.StagingGraphsync {
	graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
	loader := storeutil.LoaderForBlockstore(ibs)
	storer := storeutil.StorerForBlockstore(ibs)
	gs := graphsync.New(metrics.LifecycleCtx(mctx, lc), graphsyncNetwork, loader, storer, graphsync.RejectAllRequestsByDefault())

	return gs
}
