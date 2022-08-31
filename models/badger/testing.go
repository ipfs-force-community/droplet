package badger

import (
	"testing"

	"github.com/filecoin-project/venus-market/v2/models/repo"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) repo.Repo {
	opts := &badger.DefaultOptions
	opts.InMemory = true
	db, err := badger.NewDatastore("", opts)
	assert.Nil(t, err)

	return NewBadgerRepo(BadgerDSParams{
		FundDS:           NewFundMgrDS(db),
		StorageDealsDS:   NewStorageDealsDS(db),
		PaychDS:          NewPayChanDS(db),
		AskDS:            NewStorageAskDS(db),
		RetrAskDs:        NewRetrievalAskDS(db),
		CidInfoDs:        NewCidInfoDs(db),
		RetrievalDealsDs: NewRetrievalDealsDS(db),
	})
}
