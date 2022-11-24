package clients

import (
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus-market/v2/api/clients/signer"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/ipfs-force-community/venus-common-utils/builder"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var log = logging.Logger("clients")

var ClientsOpts = func(server bool, msgCfg *config.Messager, signerCfg *config.Signer) builder.Option {
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
			// builder.Override(new(gwAPI.IMarketEvent), NewMarketEvent),  // 部署在线下充当gateway的MarketEventStream功能,实现方式需进一步调研？
		)
	}

	return builder.Options(opts,
		builder.Override(new(v1api.FullNode), NodeClient),
	)
}
