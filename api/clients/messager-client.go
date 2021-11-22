package clients

import (
	"context"
	"github.com/filecoin-project/venus-market/config"
	client2 "github.com/filecoin-project/venus-messager/api/client"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var ErrFailMsg = xerrors.New("Message Fail")

type IMessager = client2.IMessager

func MessagerClient(mctx metrics.MetricsCtx, lc fx.Lifecycle, nodeCfg *config.Messager) (IMessager, error) {
	info := apiinfo.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	dialAddr, err := info.DialArgs("v0")
	if err != nil {
		return nil, err
	}

	client, closer, err := client2.NewMessageRPC(mctx, dialAddr, info.AuthHeader())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return client, err
}
