package clients

import (
	"github.com/filecoin-project/venus-auth/jwtclient"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	"github.com/filecoin-project/venus-market/v2/api/clients/signer"
	"github.com/filecoin-project/venus-market/v2/config"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var log = logging.Logger("clients")

var ClientsOpts = func(server bool, msgCfg *config.Messager, signerCfg *config.Signer, authClient jwtclient.IAuthClient) builder.Option {
	return builder.Options(
		builder.Override(new(IMixMessage), NewMixMsgClient),
		builder.Override(new(signer.ISigner), signer.NewISignerClient(server, authClient)),
		builder.ApplyIf(
			func(s *builder.Settings) bool {
				return len(msgCfg.Url) > 0
			},
			builder.Override(new(IVenusMessager), MessagerClient),
		),
		builder.Override(new(v1api.FullNode), NodeClient),
	)
}
