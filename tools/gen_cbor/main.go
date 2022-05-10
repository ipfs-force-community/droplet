package main

import (
	"github.com/filecoin-project/venus-market/v2/types"
	gen "github.com/whyrusleeping/cbor-gen"
)

func main() {
	if err := gen.WriteTupleEncodersToFile("./types/cbor_gen.go", "types",
		types.DealResponse{},
		types.DealStatusRequest{},
		types.DealStatusResponse{},
		types.DealStatus{},
	); err != nil {
		panic(err)
	}
}
