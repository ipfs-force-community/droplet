package models

import (
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	ds2 "github.com/ipfs/go-datastore"
)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo() repo.Repo {
	ds := ds2.NewMapDatastore()
	return badger.NewBadgerRepo(badger.BadgerDSParams{
		FundDS:           badger.NewFundMgrDS(ds),
		StorageDealsDS:   badger.NewStorageDealsDS(ds),
		PaychDS:          badger.NewPayChanDS(ds),
		AskDS:            badger.NewStorageAskDS(ds),
		RetrAskDs:        badger.NewRetrievalAskDS(ds),
		CidInfoDs:        badger.NewCidInfoDs(ds),
		RetrievalDealsDs: badger.NewRetrievalDealsDS(ds),
	})
}
