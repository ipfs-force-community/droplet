package impl

import (
	"context"
	"github.com/filecoin-project/venus-market/fundmgr"
	"github.com/filecoin-project/venus/app/client/apiface"

	"github.com/ipfs/go-cid"
	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"
	marketactor "github.com/filecoin-project/lotus/chain/actors/builtin/market"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors"
)

type FundAPI struct {
	fx.In

	Full apiface.FullNode
	FMgr *fundmgr.FundManager
}

func (a *FundAPI) MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt types.BigInt) (cid.Cid, error) {
	params, err := specactors.SerializeParams(&addr)
	if err != nil {
		return cid.Undef, err
	}

	smsg, aerr := a.Full.MpoolPushMessage(ctx, &types.UnsignedMessage{
		To:     marketactor.Address,
		From:   wallet,
		Value:  amt,
		Method: marketactor.Methods.AddBalance,
		Params: params,
	}, nil)

	if aerr != nil {
		return cid.Undef, aerr
	}

	return smsg.Cid(), nil
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
