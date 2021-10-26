package mysql

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-wallet/crypto"
	"gorm.io/gorm"
)

type ask struct {
	Miner string `gorm:"column:miner;uniqueIndex;type:varchar(128)"`

	Price         abi.TokenAmount
	VerifiedPrice abi.TokenAmount

	MinPieceSize abi.PaddedPieceSize
	MaxPieceSize abi.PaddedPieceSize
	Timestamp    abi.ChainEpoch
	Expiry       abi.ChainEpoch
	SeqNo        uint64

	Signature *crypto.Signature
	gorm.Model
}
