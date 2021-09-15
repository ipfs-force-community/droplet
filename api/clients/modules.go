package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/builder"
	types2 "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/types"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
	"time"
)

var log = logging.Logger("clients")

const (
	ReplaceMpoolMethod  builder.Invoke = 6
	ReplaceWalletMethod builder.Invoke = 7
)

func ConvertMpoolToMessager(fullNode apiface.FullNode, messager IMessager) error {
	fullNodeStruct := fullNode.(*client.FullNodeStruct)
	fullNodeStruct.IMessagePoolStruct.Internal.MpoolPushMessage = func(ctx context.Context, p1 *types.UnsignedMessage, p2 *types.MessageSendSpec) (*types.SignedMessage, error) {
		uid, err := messager.PushMessage(ctx, p1, nil)
		if err != nil {
			return nil, err
		}
		log.Infof("push message to messager %s", uid)
		for {
			msgDetail, err := messager.GetMessageByUid(ctx, uid)
			if err != nil {
				log.Errorf("get message detail from messager %w", err)
				return nil, err
			}
			switch msgDetail.State {
			case types2.UnFillMsg:
				time.Sleep(time.Second * 10)
				continue
			case types2.FailedMsg:
				return nil, xerrors.Errorf("push message %w", err)
			default:
				return &types.SignedMessage{
					Message:   msgDetail.UnsignedMessage,
					Signature: *msgDetail.Signature,
				}, nil
			}
		}
	}
	return nil
}

func ConvertWalletToISinge(fullNode apiface.FullNode, signer ISinger) error {
	fullNodeStruct := fullNode.(*client.FullNodeStruct)
	fullNodeStruct.IWalletStruct.Internal.WalletHas = func(p0 context.Context, p1 address.Address) (bool, error) {
		return signer.WalletHas(p0, p1)
	}
	return nil
}

var ClientsOpts = builder.Options(
	builder.Override(ReplaceMpoolMethod, ConvertMpoolToMessager),
	builder.Override(ReplaceWalletMethod, ConvertWalletToISinge),
)
