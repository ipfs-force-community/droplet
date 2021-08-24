package types

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-state-types/big"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

type MarketBalance struct {
	Escrow big.Int
	Locked big.Int
}

type MarketDeal = vTypes.MarketDeal

type DataTransferChannel struct {
	TransferID  datatransfer.TransferID
	Status      datatransfer.Status
	BaseCID     cid.Cid
	IsInitiator bool
	IsSender    bool
	Voucher     string
	Message     string
	OtherPeer   peer.ID
	Transferred uint64
	Stages      *datatransfer.ChannelStages
}
