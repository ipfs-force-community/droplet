package models

import (
	"testing"

	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	ds2 "github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo(t *testing.T) repo.Repo {
	ds := ds2.NewMapDatastore()
	r, err := badger.NewBadgerRepo(badger.BadgerDSParams{
		FundDS:           badger.NewFundMgrDS(ds),
		StorageDealsDS:   badger.NewStorageDealsDS(ds),
		PaychDS:          badger.NewPayChanDS(ds),
		AskDS:            badger.NewStorageAskDS(ds),
		RetrAskDs:        badger.NewRetrievalAskDS(ds),
		CidInfoDs:        badger.NewCidInfoDs(ds),
		RetrievalDealsDs: badger.NewRetrievalDealsDS(ds),
	})
	assert.Nil(t, err)
	return r
}
