package storageadapter

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
)

type StorageAsk interface {
	GetAsk(mAddr address.Address) (*storagemarket.SignedStorageAsk, error)
	SetAsk(mAddr address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error
}

// TODO: create instance
