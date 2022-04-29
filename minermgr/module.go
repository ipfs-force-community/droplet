package minermgr

import (
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/ipfs-force-community/venus-common-utils/builder"
)

var MinerMgrOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(IAddrMgr), NeAddrMgrImpl),
	)
}
