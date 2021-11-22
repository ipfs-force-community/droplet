package paychmgr

import (
	"github.com/ipfs-force-community/venus-common-utils/builder"
)

var PaychOpts = builder.Options(
	builder.Override(new(*Manager), NewManager),
	builder.Override(new(*PaychAPI), func(p *Manager) *PaychAPI {
		return NewPaychAPI(p)
	}),
)
