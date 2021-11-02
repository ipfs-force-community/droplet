package paychmgr

import (
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/repo"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
)

func newRepo() repo.Repo {
	r, err := badger.NewBadgerRepo(nil, nil, ds_sync.MutexWrap(ds.NewMapDatastore()), nil, nil, nil, nil)
	if err != nil {
		panic("new badger repo failed: " + err.Error())
	}
	return r
}
