package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/fx"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/ipfs-force-community/droplet/v2/api/clients/signer"
	"github.com/ipfs-force-community/droplet/v2/utils"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	msgTypes "github.com/filecoin-project/venus/venus-shared/types/messager"
)

var ErrMarkFailedMessageByMessager = errors.New("mark failed message by messager")

type IMixMessage interface {
	GetMessage(ctx context.Context, mid cid.Cid) (*types.Message, error)
	PushMessage(ctx context.Context, msg *types.Message, msgSendSpec *types.MessageSendSpec) (cid.Cid, error)
	GetMessageChainCid(ctx context.Context, mid cid.Cid) (*cid.Cid, error)
	WaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, loopBackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)
	SearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, loopBackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)
}

type MPoolReplaceParams struct {
	fx.In
	FullNode      v1api.FullNode
	Signer        signer.ISigner
	VenusMessager IVenusMessager `optional:"true"`
}

type MixMsgClient struct {
	full          v1api.FullNode
	venusMessager IVenusMessager
	signer        signer.ISigner
	nonceAssign   INonceAssigner
}

func NewMixMsgClient(params MPoolReplaceParams) IMixMessage {
	return &MixMsgClient{
		full:          params.FullNode,
		venusMessager: params.VenusMessager,
		signer:        params.Signer,
		nonceAssign:   newNonceAssign(params.FullNode),
	}
}

func (msgClient *MixMsgClient) PushMessage(ctx context.Context, msg *types.Message, msgSendSpec *types.MessageSendSpec) (cid.Cid, error) {
	if msgClient.venusMessager == nil {
		var sendSpec *types.MessageSendSpec
		if msgSendSpec != nil {
			sendSpec = &types.MessageSendSpec{
				MaxFee:            msgSendSpec.MaxFee,
				GasOverEstimation: msgSendSpec.GasOverEstimation,
			}
		}

		var err error
		msg.From, err = msgClient.full.StateAccountKey(ctx, msg.From, types.EmptyTSK)
		if err != nil {
			return cid.Undef, err
		}
		// estimate -> sign -> push
		estimatedMsg, err := msgClient.full.GasEstimateMessageGas(ctx, msg, sendSpec, types.EmptyTSK)
		if err != nil {
			return cid.Undef, err
		}
		estimatedMsg.Nonce, err = msgClient.nonceAssign.AssignNonce(ctx, msg.From)
		if err != nil {
			return cid.Undef, err
		}
		storageBlock, err := estimatedMsg.ToStorageBlock()
		if err != nil {
			return cid.Undef, err
		}

		sig, err := msgClient.signer.WalletSign(ctx, estimatedMsg.From, storageBlock.Cid().Bytes(), types.MsgMeta{
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

	msgID, err := utils.NewMId()
	if err != nil {
		return cid.Undef, err
	}

	// from-account-signer handling moved to sophon-gateway
	_, err = msgClient.venusMessager.PushMessageWithId(ctx, msgID.String(), msg, nil)
	if err != nil {
		return cid.Undef, err
	}

	log.Warnf("push message %s to sophon-messager", msgID.String())

	return msgID, nil
}

func (msgClient *MixMsgClient) WaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, loopbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	if msgClient.venusMessager == nil || mCid.Prefix() != utils.MidPrefix {
		return msgClient.full.StateWaitMsg(ctx, mCid, confidence, loopbackLimit, allowReplaced)
	}

	tm := time.NewTicker(time.Second * 120)
	defer tm.Stop()

	doneCh := make(chan struct{}, 1)
	doneCh <- struct{}{}

	for {
		select {
		case <-doneCh:
			msg, err := msgClient.venusMessager.GetMessageByUid(ctx, mCid.String())
			if err != nil {
				log.Warnf("get message %s fail while wait %v", mCid, err)
				time.Sleep(time.Second * 5)
				continue
			}

			switch msg.State {
			//OffChain
			case msgTypes.FillMsg:
				fallthrough
			case msgTypes.UnFillMsg:
				fallthrough
			case msgTypes.UnKnown:
				continue
			//OnChain
			case msgTypes.NonceConflictMsg:
				fallthrough
			case msgTypes.OnChainMsg:
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
			case msgTypes.FailedMsg:
				var reason string
				if msg.Receipt != nil {
					reason = string(msg.Receipt.Return)
				}
				return nil, fmt.Errorf("msg failed due to %s, %w", reason, ErrMarkFailedMessageByMessager)
			}

		case <-tm.C:
			doneCh <- struct{}{}
		case <-ctx.Done():
			return nil, fmt.Errorf("get message fail while wait")
		}
	}
}

func (msgClient *MixMsgClient) SearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, loopbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	if msgClient.venusMessager == nil || mCid.Prefix() != utils.MidPrefix {
		return msgClient.full.StateSearchMsg(ctx, from, mCid, loopbackLimit, allowReplaced)
	}

	msg, err := msgClient.venusMessager.GetMessageByUid(ctx, mCid.String())
	if err != nil {
		log.Warnw("get message fail while wait %w", err)
		time.Sleep(time.Second * 5)
		return nil, err
	}

	switch msg.State {
	//OffChain
	case msgTypes.FillMsg:
		fallthrough
	case msgTypes.UnFillMsg:
		fallthrough
	case msgTypes.UnKnown:
		return nil, nil
	//OnChain
	case msgTypes.NonceConflictMsg:
		fallthrough
	case msgTypes.OnChainMsg:
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
	case msgTypes.FailedMsg:
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
	if msgClient.venusMessager == nil || mCid.Prefix() != utils.MidPrefix {
		return msgClient.full.ChainGetMessage(ctx, mCid)
	}

	msg, err := msgClient.venusMessager.GetMessageByUid(ctx, mCid.String())
	if err != nil {
		return nil, err
	}

	return msg.VMMessage(), nil
}

func (msgClient *MixMsgClient) GetMessageChainCid(ctx context.Context, mid cid.Cid) (*cid.Cid, error) {
	if mid.Prefix() == utils.MidPrefix {
		if msgClient.venusMessager == nil {
			return nil, fmt.Errorf("unable to get message chain cid from messager,no messager configured")
		}
		msg, err := msgClient.venusMessager.GetMessageByUid(ctx, mid.String())
		if err != nil {
			return nil, err
		}

		return msg.SignedCid, nil
	}

	return &mid, nil
}
