package paychmgr

import (
	"github.com/filecoin-project/venus-market/builder"
)

var PaychOpts = builder.Options(
	builder.Override(new(*Manager), NewManager),
	builder.Override(new(*PaychAPI), func(p *Manager) *PaychAPI {
		return NewPaychAPI(p)
	}),
)
