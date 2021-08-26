package models

import (
	"github.com/filecoin-project/go-multistore"
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/venus-market/blockstore"
)

type StagingDS datastore.Batching

type StagingBlockstore blockstore.BasicBlockstore

type StagingMultiDstore *multistore.MultiStore
