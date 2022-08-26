package clients

import (
	"context"
	"errors"
	"net/http"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus/venus-shared/api/messager"
	"github.com/ipfs-force-community/metrics"
	"go.uber.org/fx"
)

var ErrFailMsg = errors.New("message fail")

type IVenusMessager = messager.IMessager

func MessagerClient(mctx metrics.MetricsCtx, lc fx.Lifecycle, messageCfg *config.Messager) (IVenusMessager, error) {
	client, closer, err := messager.DialIMessagerRPC(mctx, messageCfg.Url, messageCfg.Token, http.Header{})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})

	return client, nil
}
