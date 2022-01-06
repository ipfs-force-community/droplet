package clients

import (
	"context"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-market/config"

	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/metrics"

	"github.com/ipfs-force-community/venus-gateway/marketevent"
	types3 "github.com/ipfs-force-community/venus-gateway/types"
)

var log = logging.Logger("clients")

var (
	ReplaceWalletMethod = builder.NextInvoke()
)

func ConvertWalletToISinge(fullNode v1api.FullNode, signer ISinger) error {
	fullNodeStruct := fullNode.(*v1api.FullNodeStruct)
	fullNodeStruct.IWalletStruct.Internal.WalletHas = func(p0 context.Context, p1 address.Address) (bool, error) {
		return signer.WalletHas(p0, p1)
	}
	fullNodeStruct.IWalletStruct.Internal.WalletSign = func(p0 context.Context, p1 address.Address, p2 []byte, p3 types2.MsgMeta) (*vCrypto.Signature, error) {
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

var ClientsOpts = func(server bool, mode string, mCfg *config.Messager, signerCfg *config.Signer) builder.Option {
	opts := builder.Options(
		builder.Override(new(IMixMessage), NewMixMsgClient),
		builder.ApplyIf(
			func(s *builder.Settings) bool {
				return len(mCfg.Url) > 0
			},
			builder.Override(new(IVenusMessager), MessagerClient)),
		builder.ApplyIf(
			func(s *builder.Settings) bool {
				return len(signerCfg.SignerType) > 0 && len(signerCfg.Url) > 0
			},
			builder.Override(new(ISinger), NewISignerClient),
			builder.Override(ReplaceWalletMethod, ConvertWalletToISinge),
		),
	)

	if server {
		return builder.Options(opts,
			builder.Override(new(v1api.FullNode), NodeClient),

			builder.ApplyIf(
				func(s *builder.Settings) bool {
					return mode == "solo"
				},
				builder.Override(new(*marketevent.MarketEventStream), NewMarketEvent),
				builder.Override(new(marketevent.IMarketEventAPI), NewMarketEventAPI),
				builder.Override(new(MarketRequestEvent), NewIMarketEvent),
			),
		)
	} else {
		return builder.Options(opts,
			builder.Override(new(v1api.FullNode), NodeClient),
		)
	}
}
