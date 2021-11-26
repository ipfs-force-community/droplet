package clients

import (
	"context"
	"github.com/filecoin-project/venus-market/minermgr"
	"go.uber.org/fx"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus-market/config"

	types2 "github.com/filecoin-project/venus-messager/types"

	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/metrics"

	"github.com/ipfs-force-community/venus-gateway/marketevent"
	types3 "github.com/ipfs-force-community/venus-gateway/types"
)

var log = logging.Logger("clients")

var (
	ReplaceMpoolMethod  = builder.NextInvoke()
	ReplaceWalletMethod = builder.NextInvoke()
)

type MPoolReplaceParams struct {
	fx.In
	FullNode apiface.FullNode
	Messager IMessager
	Mgr      minermgr.IAddrMgr `optional:"true"`
}

func ConvertMpoolToMessager(params MPoolReplaceParams) error {
	fullNodeStruct := params.FullNode.(*client.FullNodeStruct)

	fullNodeStruct.IMessagePoolStruct.Internal.MpoolPushMessage = func(ctx context.Context, p1 *types.UnsignedMessage, p2 *types.MessageSendSpec) (*types.SignedMessage, error) {
		//todo use MpoolPushSignedMessage to replace MpoolPushMessage.
		// but due to louts mpool, cannt repub messager not in local, so may stuck in daemon pool
		// if this issue was fixed, should changed it
		var uid string
		var err error
		if params.Mgr != nil {
			fromAddr, err := params.FullNode.StateAccountKey(ctx, p1.From, types.EmptyTSK)
			account, err := params.Mgr.GetAccount(ctx, fromAddr)
			if err != nil {
				return nil, err
			}
			uid, err = params.Messager.ForcePushMessage(ctx, account, p1, nil)
			if err != nil {
				return nil, err
			}
		} else {
			//for client , accout has in token
			uid, err = params.Messager.PushMessage(ctx, p1, nil)
			if err != nil {
				return nil, err
			}
		}

		var showLog bool
		for {
			msgDetail, err := params.Messager.GetMessageByUid(ctx, uid)
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
				msg, err := params.Messager.GetMessageByCid(ctx, mCid)
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
		msg, err := params.Messager.GetMessageByCid(ctx, mCid)
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

	fullNodeStruct.IChainInfoStruct.Internal.ChainGetMessage = func(ctx context.Context, mid cid.Cid) (*types.UnsignedMessage, error) {
		msg, err := params.Messager.GetMessageByCid(ctx, mid)
		if err != nil {
			return fullNodeStruct.ChainGetMessage(ctx, mid)
		}
		return msg.VMMessage(), nil
	}
	return nil
}

func ConvertWalletToISinge(fullNode apiface.FullNode, signer ISinger) error {
	fullNodeStruct := fullNode.(*client.FullNodeStruct)
	fullNodeStruct.IWalletStruct.Internal.WalletHas = func(p0 context.Context, p1 address.Address) (bool, error) {
		return signer.WalletHas(p0, p1)
	}
	fullNodeStruct.IWalletStruct.Internal.WalletSign = func(p0 context.Context, p1 address.Address, p2 []byte, p3 wallet.MsgMeta) (*vCrypto.Signature, error) {
		return signer.WalletSign(p0, p1, p2, p3)
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

var ClientsOpts = func(server bool, mCfg *config.Messager, signerCfg *config.Signer) builder.Option {
	opts := builder.Options(
		builder.ApplyIf(
			func(s *builder.Settings) bool {
				return len(mCfg.Url) > 0
			},
			builder.Override(new(IMessager), MessagerClient),
			builder.Override(ReplaceMpoolMethod, ConvertMpoolToMessager)),

		builder.ApplyIf(
			func(s *builder.Settings) bool {
				return len(signerCfg.SignerType) > 0 && len(signerCfg.Url) > 0
			},
			builder.Override(new(ISinger), NewISignerClient),
			builder.Override(ReplaceWalletMethod, ConvertWalletToISinge)),
	)

	if server {
		return builder.Options(opts,
			builder.Override(new(apiface.FullNode), NodeClient),

			builder.Override(new(*marketevent.MarketEventStream), NewMarketEvent),
			builder.Override(new(marketevent.IMarketEventAPI), NewMarketEventAPI),
			builder.Override(new(MarketRequestEvent), builder.From(new(*marketevent.MarketEventStream))),
		)
	} else {
		return builder.Options(opts,
			builder.Override(new(apiface.FullNode), NodeClient),
		)
	}
}
