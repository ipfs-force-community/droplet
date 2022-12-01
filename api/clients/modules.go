package clients

import (
	logging "github.com/ipfs/go-log/v2"

	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-gateway/marketevent"

	gwTypes "github.com/ipfs-force-community/venus-gateway/types"

	"github.com/filecoin-project/venus-market/v2/api/clients/signer"
	"github.com/filecoin-project/venus-market/v2/config"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	gwAPI "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
)

var log = logging.Logger("clients")

func NewMarketEvent(mctx metrics.MetricsCtx) (gwAPI.IMarketEvent, error) {
	stream := marketevent.NewMarketEventStream(mctx, &localMinerValidator{}, gwTypes.DefaultConfig())
	return stream, nil
}

var ClientsOpts = func(server bool, mode string, msgCfg *config.Messager, signerCfg *config.Signer) builder.Option {
	opts := builder.Options(
		builder.Override(new(IMixMessage), NewMixMsgClient),
		builder.Override(new(signer.ISigner), signer.NewISignerClient(server)),
		builder.ApplyIf(
			func(s *builder.Settings) bool {
				return len(msgCfg.Url) > 0
			},
			builder.Override(new(IVenusMessager), MessagerClient)),
	)

	if server {
		return builder.Options(opts,
			builder.Override(new(v1api.FullNode), NodeClient),

			builder.ApplyIf(
				func(s *builder.Settings) bool {
					return mode == "solo"
				},
				builder.Override(new(gwAPI.IMarketEvent), NewMarketEvent),
			),
		)
	}

	return builder.Options(opts,
		builder.Override(new(v1api.FullNode), NodeClient),
	)
}
