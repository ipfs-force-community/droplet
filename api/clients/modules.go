package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/metrics"
	types2 "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
	"time"
)

var log = logging.Logger("clients")

var (
	ReplaceMpoolMethod  builder.Invoke = builder.NextInvoke()
	ReplaceWalletMethod builder.Invoke = builder.NextInvoke()
)

func ConvertMpoolToMessager(fullNode apiface.FullNode, messager IMessager) error {
	fullNodeStruct := fullNode.(*client.FullNodeStruct)
	fullNodeStruct.IMessagePoolStruct.Internal.MpoolPushMessage = func(ctx context.Context, p1 *types.UnsignedMessage, p2 *types.MessageSendSpec) (*types.SignedMessage, error) {
		uid, err := messager.PushMessage(ctx, p1, nil)
		if err != nil {
			return nil, err
		}
		var showLog bool
		for {
			msgDetail, err := messager.GetMessageByUid(ctx, uid)
			if err != nil {
				log.Errorf("get message detail from messager %w", err)
				return nil, err
			}
			if !showLog {
				log.Infof("push message to messager uid: %s, cid: %s", uid, msgDetail.Cid())
				showLog = true
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

func NewMarketEvent(mctx metrics.MetricsCtx) (*marketevent.MarketEventStream, error) {
	stream := marketevent.NewMarketEventStream(mctx, func(miner address.Address) (bool, error) {
		return true, nil
	}, nil)
	return stream, nil
}

func NewMarketEventAPI(stream *marketevent.MarketEventStream) (*marketevent.MarketEventAPI, error) {
	return marketevent.NewMarketEventAPI(stream), nil
}

func NewIMarketEvent(stream *marketevent.MarketEventStream) (MarketRequestEvent, error) {
	return stream, nil
}

var ClientsOpts = func(server bool) builder.Option {
	opts := builder.Options(
		builder.Override(ReplaceMpoolMethod, ConvertMpoolToMessager),
		builder.Override(ReplaceWalletMethod, ConvertWalletToISinge),
	)
	if server {
		return builder.Options(opts,
			builder.Override(new(apiface.FullNode), NodeClient),
			builder.Override(new(IMessager), MessagerClient),
			builder.Override(new(ISinger), NewWalletClient),

			builder.Override(new(*marketevent.MarketEventStream), NewMarketEvent),
			builder.Override(new(marketevent.IMarketEventAPI), NewMarketEventAPI),
			builder.Override(new(MarketRequestEvent), builder.From(new(*marketevent.MarketEventStream))),
		)
	} else {
		return builder.Options(opts,
			builder.Override(new(apiface.FullNode), NodeClient),
			builder.Override(new(ISinger), NewWalletClient),
			builder.Override(new(IMessager), MessagerClient))
	}
}
