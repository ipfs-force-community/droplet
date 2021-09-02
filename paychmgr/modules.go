package paychmgr

import (
	"github.com/filecoin-project/venus-market/builder"
	paych3 "github.com/filecoin-project/venus/app/submodule/paych"
	"github.com/filecoin-project/venus/pkg/paychmgr"
)

var PaychOpts = builder.Options(
	builder.Override(new(*paychmgr.Manager), NewManager),
	builder.Override(new(*paych3.PaychAPI), func(p *paychmgr.Manager) *paych3.PaychAPI {
		return paych3.NewPaychAPI(p)
	}),
)
