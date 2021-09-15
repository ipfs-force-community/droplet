package clients

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"go.uber.org/fx"
)

func NodeClient(mctx metrics.MetricsCtx, lc fx.Lifecycle, nodeCfg *config.Node) (apiface.FullNode, error) {
	fullNode := client.FullNodeStruct{}

	aInfo := apiinfo.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	addr, err := aInfo.DialArgs("v1")
	if err != nil {
		return nil, err
	}

	closer, err := jsonrpc.NewMergeClient(mctx, addr, "Filecoin", utils.GetInternalStructs(&fullNode), aInfo.AuthHeader())

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return &fullNode, err
}
