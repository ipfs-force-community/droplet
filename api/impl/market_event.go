package impl

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/types/gateway"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	"go.uber.org/fx"
)

type MarketEventAPI struct {
	fx.In

	Event marketevent.IMarketEventAPI `optional:"true"`
}

var errNotSupportGateWayMode = fmt.Errorf("MarketEvent api supported only when it runs in 'solo' mode")

func (marketEvent *MarketEventAPI) ResponseMarketEvent(ctx context.Context, resp *gateway.ResponseEvent) error {
	if marketEvent.Event == nil {
		return errNotSupportGateWayMode
	}
	return marketEvent.Event.ResponseMarketEvent(ctx, resp)
}

func (marketEvent *MarketEventAPI) ListenMarketEvent(ctx context.Context, policy *gateway.MarketRegisterPolicy) (<-chan *gateway.RequestEvent, error) {
	if marketEvent.Event == nil {
		return nil, errNotSupportGateWayMode
	}
	return marketEvent.Event.ListenMarketEvent(ctx, policy)
}
