package types

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

// ProviderDealState is the current state of a deal from the point of view
// of a retrieval provider
type ProviderDealState struct {
	retrievalmarket.DealProposal
	StoreID               uint64
	SelStorageProposalCid cid.Cid
	ChannelID             *datatransfer.ChannelID
	Status                retrievalmarket.DealStatus
	Receiver              peer.ID
	TotalSent             uint64
	FundsReceived         abi.TokenAmount
	Message               string
	CurrentInterval       uint64
	LegacyProtocol        bool
}

func (deal *ProviderDealState) IntervalLowerBound() uint64 {
	return deal.Params.IntervalLowerBound(deal.CurrentInterval)
}

func (deal *ProviderDealState) NextInterval() uint64 {
	return deal.Params.NextInterval(deal.CurrentInterval)
}

// Identifier provides a unique id for this provider deal
func (pds ProviderDealState) Identifier() retrievalmarket.ProviderDealIdentifier {
	return retrievalmarket.ProviderDealIdentifier{Receiver: pds.Receiver, DealID: pds.ID}
}
