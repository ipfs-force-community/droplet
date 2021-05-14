package blockstore

import (
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

type MetadataDS datastore.Batching

type StagingDs datastore.Batching
type StagingBlockstore blockstore.Blockstore
