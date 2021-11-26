package main

import (
	"github.com/filecoin-project/venus-market/types"
	gen "github.com/whyrusleeping/cbor-gen"
)

func main() {
	if err := gen.WriteTupleEncodersToFile("./types/cbor_gen.go", "types",
		types.FundedAddressState{},
		types.MsgInfo{},
		types.ChannelInfo{},
		types.VoucherInfo{},
		types.MinerDeal{},
		types.RetrievalAsk{},
		types.ProviderDealState{},
	); err != nil {
		panic(err)
	}
}
