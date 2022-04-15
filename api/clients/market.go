package clients

import (
	"context"

	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"go.uber.org/fx"

	marketapi "github.com/filecoin-project/venus/venus-shared/api/market"
)

func MarketClient(mctx metrics.MetricsCtx, lc fx.Lifecycle, marketCfg *config.Market) (marketapi.IMarket, error) {
	aInfo := apiinfo.NewAPIInfo(marketCfg.Url, marketCfg.Token)
	addr, err := aInfo.DialArgs("v0")
	if err != nil {
		return nil, err
	}

	api, closer, err := marketapi.NewIMarketRPC(mctx, addr, aInfo.AuthHeader())
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return api, err
}
