package constants

import "github.com/filecoin-project/go-state-types/abi"

const (
	Finality          = 900
	MessageConfidence = 5
	BlockDelaySecs    = 30
	LookbackNoLimit   = abi.ChainEpoch(-1)
)
