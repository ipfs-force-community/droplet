package indexprovider

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/net"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type NetworkName string

var IndexProviderOpts = builder.Options(
	builder.Override(new(NetworkName), func(mCtx metrics.MetricsCtx, full v1.FullNode) (NetworkName, error) {
		// nn, err := full.StateNetworkName(mCtx)
		// return NetworkName(nn), err
		return NetworkName("mainnet"), nil
	}),
	builder.Override(new(*pubsub.PubSub), NewPubSub),
	builder.Override(new(*IndexProviderMgr), NewIndexProviderMgr),
)

func NewPubSub(mCtx metrics.MetricsCtx, h host.Host, nn NetworkName) (*pubsub.PubSub, error) {
	drandSchedule := make(map[abi.ChainEpoch]config.DrandEnum)
	sk := net.NewScoreKeeper()
	return net.NewGossipSub(mCtx, h, sk, string(nn), drandSchedule, []peer.AddrInfo{}, false, false)
}
