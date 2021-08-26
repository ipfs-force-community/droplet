package models

import (
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/venus-market/blockstore"
)

type StagingDS datastore.Batching

type StagingBlockstore blockstore.BasicBlockstore
