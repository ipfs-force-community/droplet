package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/metrics"
	types2 "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	types3 "github.com/ipfs-force-community/venus-gateway/types"
	"github.com/ipfs/go-cid"
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

	fullNodeStruct.IChainInfoStruct.Internal.StateWaitMsg = func(ctx context.Context, mCid cid.Cid, confidence uint64, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
		tm := time.NewTicker(time.Second * 30)
		defer tm.Stop()

		doneCh := make(chan struct{}, 1)
		doneCh <- struct{}{}

		for {
			select {
			case <-doneCh:
				msg, err := messager.GetMessageByCid(ctx, mCid)
				if err != nil {
					log.Warnw("get message fail while wait %w", err)
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

	fullNodeStruct.IChainInfoStruct.Internal.StateSearchMsg = func(ctx context.Context, from types.TipSetKey, mCid cid.Cid, _ abi.ChainEpoch, _ bool) (*apitypes.MsgLookup, error) {
		tm := time.NewTicker(time.Second * 30)
		defer tm.Stop()

		doneCh := make(chan struct{}, 1)
		doneCh <- struct{}{}

		for {
			select {
			case <-doneCh:
				msg, err := messager.GetMessageByCid(ctx, mCid)
				if err != nil {
					log.Warnw("get message fail while wait %w", err)
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
					if msg.Confidence > int64(0) {
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
	}, &types3.Config{
		RequestQueueSize: 30,
		RequestTimeout:   time.Second * 30,
	})
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
