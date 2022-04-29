package badger

import (
	"github.com/filecoin-project/venus-market/v2/blockstore"
	"github.com/ipfs/go-datastore"
)

type ClientBlockstore blockstore.Blockstore

// TODO this should be removed.
func NewClientBlockstore() ClientBlockstore {
	// in most cases this is now unused in normal operations -- however, it's important to preserve for the IPFS use case
	return blockstore.WrapIDStore(blockstore.FromDatastore(datastore.NewMapDatastore()))
}
