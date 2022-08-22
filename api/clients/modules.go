package clients

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/v2/config"
	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	api "github.com/filecoin-project/venus/venus-shared/api/gateway/v1"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"github.com/ipfs-force-community/venus-gateway/marketevent"
	types3 "github.com/ipfs-force-community/venus-gateway/types"
	logging "github.com/ipfs/go-log/v2"
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
	stream := marketevent.NewMarketEventStream(mctx, &localMinerValidator{}, types3.DefaultConfig())
	return stream, nil
}

func NewMarketEventAPI(stream *marketevent.MarketEventStream) (api.IMarketServiceProvider, error) {
	return stream, nil
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
			builder.Override(new(ISinger), NewISignerClient(server)),
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
				builder.Override(new(api.IMarketServiceProvider), NewMarketEventAPI),
				builder.Override(new(MarketRequestEvent), NewIMarketEvent),
			),
		)
	}
	return builder.Options(opts,
		builder.Override(new(v1api.FullNode), NodeClient),
	)
}
