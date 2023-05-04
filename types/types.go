package types

import (
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

// Clock is the global clock for the system. In standard builds,
// we use a real-time clock, which maps to the `time` package.
//
// Tests that need control of time can replace this variable with
// clock.NewMock(). Always use real time for socket/stream deadlines.
var Clock = clock.NewSystemClock()

// ShutdownChan is a channel to which you send a value if you intend to shut
// down the daemon (or miner), including the node and RPC server.
type ShutdownChan chan struct{}

type ClientOfflineDeal struct {
	types.ClientDealProposal

	ProposalCID cid.Cid
	DataRoot    cid.Cid
	Message     string
	State       uint64
	DealID      uint64
	SlashEpoch  abi.ChainEpoch
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
