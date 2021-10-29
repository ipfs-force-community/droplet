package minermgr

import (
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
)

var MinerMgrOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(IMinerMgr), NewMinerMgrImpl(cfg)),
	)
}
