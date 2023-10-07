package types

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
)

// Note that the handling for deal protocol v1.2.0 and v1.2.1 is the same.
// Deal protocol v1.2.1 has a couple of new fields: SkipIPNIAnnounce and
// RemoveUnsealedCopy.
// If a client that supports deal protocol v1.2.0 sends a request to a
// boostd server that supports deal protocol v1.2.1, the DealParams struct
// will be missing these new fields.
// When the DealParams struct is unmarshalled the missing fields will be
// set to false, which maintains the previous behaviour:
// - SkipIPNIAnnounce=false:    announce deal to IPNI
// - RemoveUnsealedCopy=false:  keep unsealed copy of deal data
const DealProtocolv120ID = "/fil/storage/mk/1.2.0"
const DealProtocolv121ID = "/fil/storage/mk/1.2.1"
const DealStatusV12ProtocolID = "/fil/storage/status/1.2.0"

// Clock is the global clock for the system. In standard builds,
// we use a real-time clock, which maps to the `time` package.
//
// Tests that need control of time can replace this variable with
// clock.NewMock(). Always use real time for socket/stream deadlines.
var Clock = clock.NewSystemClock()

// ShutdownChan is a channel to which you send a value if you intend to shut
// down the daemon (or miner), including the node and RPC server.
type ShutdownChan chan struct{}

// HttpRequest has parameters for an HTTP transfer
type HttpRequest struct {
	// URL can be
	// - an http URL:
	//   "https://example.com/path"
	// - a libp2p URL:
	//   "libp2p:///ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	//   Must include a Peer ID
	URL string
	// Headers are the HTTP headers that are sent as part of the request,
	// eg "Authorization"
	Headers map[string]string
}

// DealStatusRequest is sent to get the current state of a deal from a
// storage provider
type DealStatusRequest struct {
	DealUUID  uuid.UUID
	Signature crypto.Signature
}

// DealStatusResponse is the current state of a deal
type DealStatusResponse struct {
	DealUUID uuid.UUID
	// Error is non-empty if there is an error getting the deal status
	// (eg invalid request signature)
	Error          string
	DealStatus     *DealStatus
	IsOffline      bool
	TransferSize   uint64
	NBytesReceived uint64
}

type DealStatus struct {
	// Error is non-empty if the deal is in the error state
	Error string
	// Status is a string corresponding to a deal checkpoint
	Status string
	// SealingStatus is the sealing status reported by lotus miner
	SealingStatus string
	// Proposal is the deal proposal
	Proposal types.DealProposal
	// SignedProposalCid is the cid of the client deal proposal + signature
	SignedProposalCid cid.Cid
	// PublishCid is the cid of the Publish message sent on chain, if the deal
	// has reached the publish stage
	PublishCid *cid.Cid
	// ChainDealID is the id of the deal in chain state
	ChainDealID abi.DealID
}

type DealParams struct {
	DealUUID           uuid.UUID
	IsOffline          bool
	ClientDealProposal types.ClientDealProposal
	DealDataRoot       cid.Cid
	Transfer           Transfer // Transfer params will be the zero value if this is an offline deal
	RemoveUnsealedCopy bool
	SkipIPNIAnnounce   bool
}

// Transfer has the parameters for a data transfer
type Transfer struct {
	// The type of transfer eg "http"
	Type string
	// An optional ID that can be supplied by the client to identify the deal
	ClientID string
	// A byte array containing marshalled data specific to the transfer type
	// eg a JSON encoded struct { URL: "<url>", Headers: {...} }
	Params []byte
	// The size of the data transferred in bytes
	Size uint64
}

type DealResponse struct {
	Accepted bool
	// Message is the reason the deal proposal was rejected. It is empty if
	// the deal was accepted.
	Message string
}
