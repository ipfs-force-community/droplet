package badger

import (
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/venus-market/v2/blockstore"
)

type StagingDS datastore.Batching

type StagingBlockstore blockstore.BasicBlockstore
