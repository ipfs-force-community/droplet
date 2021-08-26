package network

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/ipfs/go-graphsync"
)

// ProviderDataTransfer is a data transfer manager for the provider
type ProviderDataTransfer datatransfer.Manager

type StagingGraphsync graphsync.GraphExchange
