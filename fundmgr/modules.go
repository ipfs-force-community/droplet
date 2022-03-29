package fundmgr

import "github.com/ipfs-force-community/venus-common-utils/builder"

var FundMgrOpts = builder.Override(new(*FundManager), NewFundManager)
