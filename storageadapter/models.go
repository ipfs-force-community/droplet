package storageadapter

import (
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

type StorageDealStore interface {
	SaveDeal(deal *storagemarket.MinerDeal) error
	GetDeal(cid cid.Cid) (*storagemarket.MinerDeal, error)
}

var RecordNotFound = fmt.Errorf("unable to find record")

type StorageAsk interface {
	GetAsk(mAddr address.Address) (*storagemarket.SignedStorageAsk, error)
	SetAsk(mAddr address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch) error
}
