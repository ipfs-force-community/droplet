package paychmgr

import (
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus/pkg/paychmgr"
)

var PaychOpts = builder.Options(
	builder.Override(new(paychmgr.Manager), NewManager),
)
