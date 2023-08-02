package clients

import (
	"context"

	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"

	"github.com/ipfs-force-community/droplet/v2/config"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs-force-community/metrics"
)

func NodeClient(mctx metrics.MetricsCtx, lc fx.Lifecycle, nodeCfg *config.Node) (v1api.FullNode, error) {
	fullNode, closer, err := v1api.DialFullNodeRPC(mctx, nodeCfg.Url, nodeCfg.Token, nil)
	if err != nil {
		return nil, err
	}

	netName, err := fullNode.StateNetworkName(mctx)
	if err != nil {
		return nil, err
	}
	if netName == types.NetworkNameMain {
		address.CurrentNetwork = address.Mainnet
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return fullNode, err
}
