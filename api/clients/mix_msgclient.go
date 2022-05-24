package clients

import (
	"context"
	"fmt"
	"time"

	types2 "github.com/filecoin-project/venus/venus-shared/types/messager"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/minermgr"
	"github.com/filecoin-project/venus-market/v2/utils"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"go.uber.org/fx"
)

type IMixMessage interface {
	GetMessage(ctx context.Context, mid cid.Cid) (*types.Message, error)
	PushMessage(ctx context.Context, p1 *types.Message, p2 *types.MessageSendSpec) (cid.Cid, error)
	GetMessageChainCid(ctx context.Context, mid cid.Cid) (*cid.Cid, error)
	WaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, loopBackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)
	SearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, loopBackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)
}

type MPoolReplaceParams struct {
	fx.In
	FullNode      v1api.FullNode
	Singer        ISinger           `optional:"true"`
	VenusMessager IVenusMessager    `optional:"true"`
	Mgr           minermgr.IAddrMgr `optional:"true"`
}

type MixMsgClient struct {
	full        v1api.FullNode
	messager    IVenusMessager
	addrMgr     minermgr.IAddrMgr
	signer      ISinger
	nonceAssign *nonceAssigner
}

func NewMixMsgClient(params MPoolReplaceParams) IMixMessage {
	return &MixMsgClient{
		full:        params.FullNode,
		messager:    params.VenusMessager,
		addrMgr:     params.Mgr,
		signer:      params.Singer,
		nonceAssign: newNonceAssign(params.FullNode),
	}
}
func (msgClient *MixMsgClient) PushMessage(ctx context.Context, p1 *types.Message, p2 *types.MessageSendSpec) (cid.Cid, error) {
	if msgClient.messager == nil {
		var sendSpec *types.MessageSendSpec
		if p2 != nil {
			sendSpec = &types.MessageSendSpec{
				MaxFee:            p2.MaxFee,
				GasOverEstimation: p2.GasOverEstimation,
			}
		}
		var err error
		p1.From, err = msgClient.full.StateAccountKey(ctx, p1.From, types.EmptyTSK)
		if err != nil {
			return cid.Undef, err
		}
		//estiamte -> sign -> push
		estimatedMsg, err := msgClient.full.GasEstimateMessageGas(ctx, p1, sendSpec, types.EmptyTSK)
		if err != nil {
			return cid.Undef, err
		}
		estimatedMsg.Nonce, err = msgClient.nonceAssign.AssignNonce(ctx, p1.From)
		if err != nil {
			return cid.Undef, err
		}
		storageBlock, err := estimatedMsg.ToStorageBlock()
		if err != nil {
			return cid.Undef, err
		}
		sig, err := msgClient.full.WalletSign(ctx, estimatedMsg.From, storageBlock.Cid().Bytes(), types.MsgMeta{
			Type:  types.MTChainMsg,
			Extra: storageBlock.RawData(),
		})
		if err != nil {
			return cid.Undef, err
		}
		signedCid, err := msgClient.full.MpoolPush(ctx, &types.SignedMessage{
			Message:   *estimatedMsg,
			Signature: *sig,
		})
		if err != nil {
			return cid.Undef, err
		}
		log.Warnf("push message %s to daemon", signedCid.String())
		return signedCid, nil
	}
	msgid, err := utils.NewMId()
	if err != nil {
		return cid.Undef, err
	}
	if msgClient.addrMgr != nil {
		fromAddr, err := msgClient.full.StateAccountKey(ctx, p1.From, types.EmptyTSK)
		if err != nil {
			return cid.Undef, err
		}
		account, err := msgClient.addrMgr.GetAccount(ctx, fromAddr)
		if err != nil {
			return cid.Undef, err
		}
		_, err = msgClient.messager.ForcePushMessageWithId(ctx, account, msgid.String(), p1, nil)
		if err != nil {
			return cid.Undef, err
		}
	} else {
		//for client account has in token
		_, err = msgClient.messager.PushMessageWithId(ctx, msgid.String(), p1, nil)
		if err != nil {
			return cid.Undef, err
		}
	}

	log.Warnf("push message %s to venus-messager", msgid.String())
	return msgid, nil
}

func (msgClient *MixMsgClient) WaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, loopbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	if msgClient.messager == nil || mCid.Prefix() != utils.MidPrefix {
		return msgClient.full.StateWaitMsg(ctx, mCid, confidence, loopbackLimit, allowReplaced)
	}
	tm := time.NewTicker(time.Second * 30)
	defer tm.Stop()

	doneCh := make(chan struct{}, 1)
	doneCh <- struct{}{}

	for {
		select {
		case <-doneCh:
			msg, err := msgClient.messager.GetMessageByUid(ctx, mCid.String())
			if err != nil {
				log.Warnf("get message %s fail while wait %v", mCid, err)
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
					return &types.MsgLookup{
						Message: mCid,
						Receipt: types.MessageReceipt{
							ExitCode: msg.Receipt.ExitCode,
							Return:   msg.Receipt.Return,
							GasUsed:  msg.Receipt.GasUsed,
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
					reason = string(msg.Receipt.Return)
				}
				return nil, fmt.Errorf("msg failed due to %s", reason)
			}

		case <-tm.C:
			doneCh <- struct{}{}
		case <-ctx.Done():
			return nil, fmt.Errorf("get message fail while wait")
		}
	}
}

func (msgClient *MixMsgClient) SearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, loopbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	if msgClient.messager == nil || mCid.Prefix() != utils.MidPrefix {
		return msgClient.full.StateSearchMsg(ctx, from, mCid, loopbackLimit, allowReplaced)
	}
	msg, err := msgClient.messager.GetMessageByUid(ctx, mCid.String())
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
		return &types.MsgLookup{
			Message: mCid,
			Receipt: types.MessageReceipt{
				ExitCode: msg.Receipt.ExitCode,
				Return:   msg.Receipt.Return,
				GasUsed:  msg.Receipt.GasUsed,
			},
			TipSet: msg.TipSetKey,
			Height: abi.ChainEpoch(msg.Height),
		}, nil
	//Error
	case types2.FailedMsg:
		var reason string
		if msg.Receipt != nil {
			reason = string(msg.Receipt.Return)
		}
		return nil, fmt.Errorf("msg failed due to %s", reason)
	default:
		return nil, fmt.Errorf("unexpect status for %v", msg.State)
	}
}

func (msgClient *MixMsgClient) GetMessage(ctx context.Context, mCid cid.Cid) (*types.Message, error) {
	if msgClient.messager == nil || mCid.Prefix() != utils.MidPrefix {
		return msgClient.full.ChainGetMessage(ctx, mCid)
	}
	msg, err := msgClient.messager.GetMessageByUid(ctx, mCid.String())
	if err != nil {
		return nil, err
	}
	return msg.VMMessage(), nil
}

func (msgClient *MixMsgClient) GetMessageChainCid(ctx context.Context, mid cid.Cid) (*cid.Cid, error) {
	if mid.Prefix() == utils.MidPrefix {
		if msgClient.messager == nil {
			return nil, fmt.Errorf("unable to get message chain cid from messager,no messager configured")
		}
		msg, err := msgClient.messager.GetMessageByUid(ctx, mid.String())
		if err != nil {
			return nil, err
		}
		return msg.SignedCid, nil
	}
	return &mid, nil
}
