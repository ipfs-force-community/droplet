package models

import (
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo() repo.Repo {
	repo, _ := badger.NewMemRepo()
	return repo
}
