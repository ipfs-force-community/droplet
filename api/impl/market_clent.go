package impl

import (
	"context"
	"github.com/filecoin-project/venus-market/api"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/client"
	mTypes "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/constants"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

var _ api.MarketClientNode = (*MarketClientNodeImpl)(nil)

type MarketClientNodeImpl struct {
	client.API
	FundAPI
	Messager clients2.IMixMessage
}

func (m *MarketClientNodeImpl) MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*apitypes.MsgLookup, error) {
	//WaitMsg method has been replace in messager mode
	return m.Messager.WaitMsg(ctx, mid, constants.MessageConfidence, constants.LookbackNoLimit, false)
}

func (m *MarketClientNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (cid.Cid, error) {
	var spec *mTypes.MsgMeta
	if meta != nil {
		spec = &mTypes.MsgMeta{
			MaxFee:            meta.MaxFee,
			GasOverEstimation: meta.GasOverEstimation,
		}
	}
	return m.Messager.PushMessage(ctx, msg, spec)
}

func (m *MarketClientNodeImpl) MessagerGetMessage(ctx context.Context, mid cid.Cid) (*vTypes.Message, error) {
	//ChainGetMessage method has been replace in messager mode
	return m.Messager.GetMessage(ctx, mid)
}
