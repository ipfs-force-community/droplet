package storageadapter

import (
	"fmt"
	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
)

var RecordNotFound = fmt.Errorf("unable to find record")

type MinerDealStore interface {
	Save(deal *storagemarket.MinerDeal) error
	Get(cid cid.Cid) (*storagemarket.MinerDeal, error)
	List(mAddr address.Address, out interface{}) error
}

// TODO: create instance
