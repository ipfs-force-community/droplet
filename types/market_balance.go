package types

import (
	"github.com/filecoin-project/go-state-types/big"
)

type MarketBalance struct {
	Escrow big.Int
	Locked big.Int
}
