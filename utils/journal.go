package utils

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/ipfs-force-community/venus-common-utils/journal"
)

// StorageProviderJournaler records journal events from the piecestorage provider.
func StorageProviderJournaler(j journal.Journal, evtType journal.EventType) func(event storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
	return func(event storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
		j.RecordEvent(evtType, func() interface{} {
			return StorageProviderEvt{
				Event: storagemarket.ProviderEvents[event],
				Deal:  deal,
			}
		})
	}
}

type StorageProviderEvt struct {
	Event string
	Deal  storagemarket.MinerDeal
}

type RetrievalProviderEvt struct {
	Event string
	Deal  types.ProviderDealState
}

// RetrievalProviderJournaler records journal events from the retrieval provider.
func RetrievalProviderJournaler(j journal.Journal, evtType journal.EventType) func(event retrievalmarket.ProviderEvent, deal types.ProviderDealState) {
	return func(event retrievalmarket.ProviderEvent, deal types.ProviderDealState) {
		j.RecordEvent(evtType, func() interface{} {
			return RetrievalProviderEvt{
				Event: retrievalmarket.ProviderEvents[event],
				Deal:  deal,
			}
		})
	}
}
