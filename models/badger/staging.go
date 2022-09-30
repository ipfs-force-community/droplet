package badger

import (
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/venus/venus-shared/blockstore"
)

type StagingDS datastore.Batching

type StagingBlockstore blockstore.BasicBlockstore
