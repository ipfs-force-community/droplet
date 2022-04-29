package impl

import (
	"context"

	"github.com/filecoin-project/venus-market/v2/api/clients"
	"github.com/filecoin-project/venus-market/v2/fundmgr"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"

	"github.com/ipfs/go-cid"
	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/actors"
	marketactor "github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type FundAPI struct {
	fx.In

	Full      v1api.FullNode
	MsgClient clients.IMixMessage
	FMgr      *fundmgr.FundManager
}

func (a *FundAPI) MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt types.BigInt) (cid.Cid, error) {
	params, err := actors.SerializeParams(&addr)
	if err != nil {
		return cid.Undef, err
	}

	msgId, aerr := a.MsgClient.PushMessage(ctx, &types.Message{
		To:     marketactor.Address,
		From:   wallet,
		Value:  amt,
		Method: marketactor.Methods.AddBalance,
		Params: params,
	}, nil)

	if aerr != nil {
		return cid.Undef, aerr
	}

	return msgId, nil
}

func (a *FundAPI) MarketGetReserved(ctx context.Context, addr address.Address) (types.BigInt, error) {
	return a.FMgr.GetReserved(addr), nil
}

func (a *FundAPI) MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt types.BigInt) (cid.Cid, error) {
	return a.FMgr.Reserve(ctx, wallet, addr, amt)
}

func (a *FundAPI) MarketReleaseFunds(ctx context.Context, addr address.Address, amt types.BigInt) error {
	return a.FMgr.Release(addr, amt)
}

func (a *FundAPI) MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt types.BigInt) (cid.Cid, error) {
	return a.FMgr.Withdraw(ctx, wallet, addr, amt)
}
