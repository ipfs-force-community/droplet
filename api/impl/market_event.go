package impl

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types/gateway"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

type MarketEventAPI struct {
	fx.In

	Event marketevent.IMarketEventAPI `optional:"true"`
}

func (marketEvent *MarketEventAPI) ResponseMarketEvent(ctx context.Context, resp *gateway.ResponseEvent) error {
	if marketEvent.Event == nil {
		return xerrors.Errorf("unsupport in gateway model")
	}
	return marketEvent.Event.ResponseMarketEvent(ctx, resp)
}

func (marketEvent *MarketEventAPI) ListenMarketEvent(ctx context.Context, policy *gateway.MarketRegisterPolicy) (<-chan *gateway.RequestEvent, error) {
	if marketEvent.Event == nil {
		return nil, xerrors.Errorf("unsupport in gateway model")
	}
	return marketEvent.Event.ListenMarketEvent(ctx, policy)
}
