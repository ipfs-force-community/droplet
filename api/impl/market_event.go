package impl

import (
	"context"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	"github.com/ipfs-force-community/venus-gateway/types"
	xerrors "github.com/pkg/errors"
	"go.uber.org/fx"
)

type MarketEventAPI struct {
	fx.In

	Event marketevent.IMarketEventAPI `optional:"true"`
}

func (marketEvent *MarketEventAPI) ResponseMarketEvent(ctx context.Context, resp *types.ResponseEvent) error {
	if marketEvent.Event == nil {
		return xerrors.Errorf("unsupport in gateway model")
	}
	return marketEvent.Event.ResponseMarketEvent(ctx, resp)
}

func (marketEvent *MarketEventAPI) ListenMarketEvent(ctx context.Context, policy *marketevent.MarketRegisterPolicy) (<-chan *types.RequestEvent, error) {
	if marketEvent.Event == nil {
		return nil, xerrors.Errorf("unsupport in gateway model")
	}
	return marketEvent.Event.ListenMarketEvent(ctx, policy)
}
