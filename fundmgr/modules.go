package fundmgr

import "github.com/filecoin-project/venus-market/builder"

var FundMgrOpts = builder.Option(
	builder.Override(new(*FundManager), NewFundManager),
)
