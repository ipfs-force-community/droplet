package models

import (
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/stretchr/testify/assert"
	"testing"
)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo(t *testing.T) repo.Repo {
	repo, err := badger.NewMemRepo()
	assert.NoError(t, err)
	return repo
}
