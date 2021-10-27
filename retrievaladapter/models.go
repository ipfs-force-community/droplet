package retrievaladapter

import (
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/libp2p/go-libp2p-core/peer"
)

type RetrievalDealStore interface {
	SaveDeal(deal *retrievalmarket.ProviderDealState) error
	GetDeal(peer.ID, retrievalmarket.DealID) (*retrievalmarket.ProviderDealState, error)
	HasDeal(peer.ID, retrievalmarket.DealID) (bool, error)
	ListDeals() ([]*retrievalmarket.ProviderDealState, error)
}

var RecordNotFound = fmt.Errorf("unable to find record")

type RetrievalAsk interface {
	GetAsk(mAddr address.Address) (*retrievalmarket.Ask, error)
	SetAsk(*retrievalmarket.Ask) error
}
