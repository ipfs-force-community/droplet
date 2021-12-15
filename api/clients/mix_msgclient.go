package clients

import (
	"context"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/utils"
	types2 "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
	"go.uber.org/fx"
)

type IMixMessage interface {
	GetMessage(ctx context.Context, mid cid.Cid) (*types.UnsignedMessage, error)
	PushMessage(ctx context.Context, p1 *types.UnsignedMessage, p2 *types2.MsgMeta) (cid.Cid, error)
	WaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, loopBackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error)
	SearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, loopBackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error)
}

type MPoolReplaceParams struct {
	fx.In
	FullNode      apiface.FullNode
	VenusMessager IVenusMessager    `optional:"true"`
	Mgr           minermgr.IAddrMgr `optional:"true"`
}

type MixMsgClient struct {
	FullNode apiface.FullNode
	Messager IVenusMessager    `optional:"true"`
	Mgr      minermgr.IAddrMgr `optional:"true"`
}

func NewMixMsgClient(params MPoolReplaceParams) IMixMessage {
	return &MixMsgClient{
		FullNode: params.FullNode,
		Messager: params.VenusMessager,
		Mgr:      params.Mgr,
	}
}
func (msgClient *MixMsgClient) PushMessage(ctx context.Context, p1 *types.UnsignedMessage, p2 *types2.MsgMeta) (cid.Cid, error) {
	if msgClient.Messager == nil {
		signed, err := msgClient.FullNode.MpoolPushMessage(ctx, p1, &types.MessageSendSpec{
			MaxFee:            p2.MaxFee,
			GasOverEstimation: p2.GasOverEstimation,
		})
		if err != nil {
			return cid.Undef, err
		}
		log.Warnf("push message %s to daemon", signed.Cid().String())
		return signed.Cid(), nil
	} else {
		msgid, err := utils.NewMId()
		if err != nil {
			return cid.Undef, err
		}
		if msgClient.Mgr != nil {
			fromAddr, err := msgClient.FullNode.StateAccountKey(ctx, p1.From, types.EmptyTSK)
			if err != nil {
				return cid.Undef, err
			}
			account, err := msgClient.Mgr.GetAccount(ctx, fromAddr)
			if err != nil {
				return cid.Undef, err
			}
			_, err = msgClient.Messager.ForcePushMessageWithId(ctx, account, msgid.String(), p1, nil)
			if err != nil {
				return cid.Undef, err
			}
		} else {
			//for client account has in token
			_, err = msgClient.Messager.PushMessageWithId(ctx, msgid.String(), p1, nil)
			if err != nil {
				return cid.Undef, err
			}
		}

		log.Warnf("push message %s to venus-messager", msgid.String())
		return msgid, nil
	}
}

func (msgClient *MixMsgClient) WaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, loopbackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	if msgClient.Messager == nil {
		return msgClient.FullNode.StateWaitMsg(ctx, mCid, confidence, loopbackLimit, allowReplaced)
	} else {
		tm := time.NewTicker(time.Second * 30)
		defer tm.Stop()

		doneCh := make(chan struct{}, 1)
		doneCh <- struct{}{}

		for {
			select {
			case <-doneCh:
				msg, err := msgClient.Messager.GetMessageByUid(ctx, mCid.String())
				if err != nil {
					log.Warnf("get message %s fail while wait %w", mCid, err)
					time.Sleep(time.Second * 5)
					continue
				}

				switch msg.State {
				//OffChain
				case types2.FillMsg:
					fallthrough
				case types2.UnFillMsg:
					fallthrough
				case types2.UnKnown:
					continue
				//OnChain
				case types2.ReplacedMsg:
					fallthrough
				case types2.OnChainMsg:
					if msg.Confidence > int64(confidence) {
						return &apitypes.MsgLookup{
							Message: mCid,
							Receipt: types.MessageReceipt{
								ExitCode:    msg.Receipt.ExitCode,
								ReturnValue: msg.Receipt.ReturnValue,
								GasUsed:     msg.Receipt.GasUsed,
							},
							TipSet: msg.TipSetKey,
							Height: abi.ChainEpoch(msg.Height),
						}, nil
					}
					continue
				//Error
				case types2.FailedMsg:
					var reason string
					if msg.Receipt != nil {
						reason = string(msg.Receipt.ReturnValue)
					}
					return nil, xerrors.Errorf("msg failed due to %s", reason)
				}

			case <-tm.C:
				doneCh <- struct{}{}
			case <-ctx.Done():
				return nil, xerrors.Errorf("get message fail while wait")
			}
		}
	}
}

func (msgClient *MixMsgClient) SearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, loopbackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	if msgClient.Messager == nil {
		return msgClient.FullNode.StateSearchMsg(ctx, from, mCid, loopbackLimit, allowReplaced)
	} else {
		msg, err := msgClient.Messager.GetMessageByCid(ctx, mCid)
		if err != nil {
			log.Warnw("get message fail while wait %w", err)
			time.Sleep(time.Second * 5)
			return nil, err
		}

		switch msg.State {
		//OffChain
		case types2.FillMsg:
			fallthrough
		case types2.UnFillMsg:
			fallthrough
		case types2.UnKnown:
			return nil, nil
		//OnChain
		case types2.ReplacedMsg:
			fallthrough
		case types2.OnChainMsg:
			return &apitypes.MsgLookup{
				Message: mCid,
				Receipt: types.MessageReceipt{
					ExitCode:    msg.Receipt.ExitCode,
					ReturnValue: msg.Receipt.ReturnValue,
					GasUsed:     msg.Receipt.GasUsed,
				},
				TipSet: msg.TipSetKey,
				Height: abi.ChainEpoch(msg.Height),
			}, nil
		//Error
		case types2.FailedMsg:
			var reason string
			if msg.Receipt != nil {
				reason = string(msg.Receipt.ReturnValue)
			}
			return nil, xerrors.Errorf("msg failed due to %s", reason)
		default:
			return nil, xerrors.Errorf("unexpect status for %v", msg.State)
		}
	}
}

func (msgClient *MixMsgClient) GetMessage(ctx context.Context, mid cid.Cid) (*types.UnsignedMessage, error) {
	if msgClient.Messager == nil {
		return msgClient.FullNode.ChainGetMessage(ctx, mid)
	} else {
		msg, err := msgClient.Messager.GetMessageByUid(ctx, mid.String())
		if err != nil {
			return nil, err
		}
		return msg.VMMessage(), nil
	}
}
