package paychmgr

import (
	"context"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	types2 "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/pkg/types"
	"golang.org/x/xerrors"
)

type MessagePullAdapter struct {
	messager clients2.IMessager
}

func (m MessagePullAdapter) MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec) (*types.SignedMessage, error) {
	id, err := m.messager.PushMessage(ctx, msg, &types2.MsgMeta{
		GasOverEstimation: spec.GasOverEstimation,
		MaxFee:            spec.MaxFee,
	})
	if err != nil {
		return nil, err
	}

	for {
		msg, err := m.messager.GetMessageByUid(ctx, id)
		if err != nil {
			return nil, err
		}
		switch msg.State {
		case types2.UnFillMsg:
			continue
		case types2.FailedMsg:
			return nil, xerrors.Errorf("msg has mark as bad %s", string(msg.Receipt.ReturnValue))
		default:
			return &types.SignedMessage{
				Message:   msg.UnsignedMessage,
				Signature: *msg.Signature,
			}, nil
		}

	}

}
