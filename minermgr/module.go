package minermgr

import (
	"context"

	"github.com/filecoin-project/go-address"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	marketTypes "github.com/filecoin-project/venus/venus-shared/types/market"
)

type IMinerMgr interface {
	Has(context.Context, address.Address) bool
	ActorList(ctx context.Context) ([]marketTypes.User, error)
}

var MinerMgrOpts = func() builder.Option {
	return builder.Options(
		builder.Override(new(IMinerMgr), NewMinerMgrImpl),
	)
}
