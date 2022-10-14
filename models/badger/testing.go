package badger

import (
	"testing"

	"github.com/filecoin-project/venus-market/v2/models/repo"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) repo.Repo {
	repo, err := NewMemRepo()
	assert.Nil(t, err)
	return repo
}

func NewMemRepo() (repo.Repo, error) {
	opts := &badger.DefaultOptions
	opts.InMemory = true
	db, err := badger.NewDatastore("", opts)
	if err != nil {
		return nil, err
	}
	payChDs := NewPayChanDS(db)
	return NewBadgerRepo(BadgerDSParams{
		FundDS:           NewFundMgrDS(db),
		StorageDealsDS:   NewStorageDealsDS(NewStorageProviderDS(db)),
		PaychInfoDS:      NewPayChanInfoDs(payChDs),
		PaychMsgDS:       NewPayChanMsgDs(payChDs),
		AskDS:            NewStorageAskDS(NewStorageProviderDS(db)),
		RetrAskDs:        NewRetrievalAskDS(NewRetrievalProviderDS(db)),
		CidInfoDs:        NewCidInfoDs(NewPieceMetaDs(db)),
		RetrievalDealsDs: NewRetrievalDealsDS(NewRetrievalProviderDS(db)),
	}), nil
}
