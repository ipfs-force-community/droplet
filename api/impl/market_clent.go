package impl

import (
	"context"
	"github.com/filecoin-project/venus-market/api"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/client"
	"github.com/filecoin-project/venus/pkg/constants"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

var _ api.MarketClientNode = (*MarketClientNodeImpl)(nil)

type MarketClientNodeImpl struct {
	client.API
	FundAPI
	Messager clients2.IMixMessage
}

func (m *MarketClientNodeImpl) MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*vTypes.MsgLookup, error) {
	//WaitMsg method has been replace in messager mode
	return m.Messager.WaitMsg(ctx, mid, constants.MessageConfidence, constants.LookbackNoLimit, false)
}

func (m *MarketClientNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *vTypes.MessageSendSpec) (cid.Cid, error) {
	var spec *vTypes.MessageSendSpec
	if meta != nil {
		spec = &vTypes.MessageSendSpec{
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
