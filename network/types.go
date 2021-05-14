package network

import (
	"github.com/ipfs/go-graphsync"
	format "github.com/ipfs/go-ipld-format"
)

type StagingDAG format.DAGService
type StagingGraphsync graphsync.GraphExchange
