package minermgr

import (
	"context"

	"github.com/filecoin-project/go-address"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	marketTypes "github.com/filecoin-project/venus/venus-shared/types/market"
)

// todo 支持动态配置?

type IMinerMgr interface {
	Has(context.Context, address.Address) bool
	ActorList(context.Context) ([]marketTypes.User, error)
	ActorUpsert(context.Context, marketTypes.User) (bool, error)
}

var MinerMgrOpts = func() builder.Option {
	return builder.Options(
		builder.Override(new(IMinerMgr), NewMinerMgrImpl),
	)
}
