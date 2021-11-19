package storageprovider

import "github.com/filecoin-project/go-fil-markets/storagemarket"

var StringToStorageState = map[string]storagemarket.StorageDealStatus{}

func init() {
	for state, stateStr := range storagemarket.DealStates {
		StringToStorageState[stateStr] = state
	}
}
