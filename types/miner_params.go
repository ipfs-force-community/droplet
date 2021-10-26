package types

import (
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	mtypes "github.com/filecoin-project/venus-messager/types"
)

type MinerParams struct {
	Miner         address.Address
	Price         mtypes.Int
	VerifiedPrice mtypes.Int
	Duration      abi.ChainEpoch
	MinPieceSize  int64
	MaxPieceSize  int64

	SignerToken string

	CreatedAt time.Time
	UpdateAt  time.Time
}
