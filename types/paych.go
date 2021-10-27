package types

// copy from github.com/filecoin-project/venus/pkg/paychmgr/store.go

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
	"github.com/ipfs/go-cid"
)

type VoucherInfo struct {
	Voucher   *paych.SignedVoucher
	Proof     []byte // ignored
	Submitted bool
}

type VoucherInfos []*VoucherInfo

func (info *VoucherInfos) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value must be []byte")
	}
	return json.Unmarshal(data, info)
}

func (info VoucherInfos) Value() (driver.Value, error) {
	return json.Marshal(info)
}

// ChannelInfo keeps track of information about a channel
type ChannelInfo struct {
	// ChannelID is a uuid set at channel creation
	ChannelID string
	// Channel address - may be nil if the channel hasn't been created yet
	Channel *address.Address
	// Control is the address of the local node
	Control address.Address
	// Target is the address of the remote node (on the other end of the channel)
	Target address.Address
	// Direction indicates if the channel is inbound (Control is the "to" address)
	// or outbound (Control is the "from" address)
	Direction uint64
	// Vouchers is a list of all vouchers sent on the channel
	Vouchers []*VoucherInfo
	// NextLane is the number of the next lane that should be used when the
	// client requests a new lane (eg to create a voucher for a new deal)
	NextLane uint64
	// Amount added to the channel.
	// Note: This amount is only used by GetPaych to keep track of how much
	// has locally been added to the channel. It should reflect the channel's
	// Balance on chain as long as all operations occur on the same datastore.
	Amount fbig.Int
	// PendingAmount is the amount that we're awaiting confirmation of
	PendingAmount fbig.Int
	// CreateMsg is the CID of a pending create message (while waiting for confirmation)
	CreateMsg *cid.Cid
	// AddFundsMsg is the CID of a pending add funds message (while waiting for confirmation)
	AddFundsMsg *cid.Cid
	// Settling indicates whether the channel has entered into the settling state
	Settling bool
}

// MsgInfo stores information about a create channel / add funds message
// that has been sent
type MsgInfo struct {
	// ChannelID links the message to a channel
	ChannelID string
	// MsgCid is the CID of the message
	MsgCid cid.Cid
	// Received indicates whether a response has been received
	Received bool
	// Err is the error received in the response
	Err string
}

const (
	DirInbound  = 1
	DirOutbound = 2
)

var ErrChannelNotTracked = xerrors.New("channel not tracked")
