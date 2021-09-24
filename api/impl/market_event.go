package impl

import (
	"context"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	"github.com/ipfs-force-community/venus-gateway/types"
	"go.uber.org/fx"
)

type MarketEventAPI struct {
	fx.In

	Event marketevent.IMarketEventAPI
}

func (marketEvent *MarketEventAPI) ResponseMarketEvent(ctx context.Context, resp *types.ResponseEvent) error {
	return marketEvent.Event.ResponseMarketEvent(ctx, resp)
}

func (marketEvent *MarketEventAPI) ListenMarketEvent(ctx context.Context, policy *marketevent.MarketRegisterPolicy) (<-chan *types.RequestEvent, error) {
	return marketEvent.Event.ListenMarketEvent(ctx, policy)
}
