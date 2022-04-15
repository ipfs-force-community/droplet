package types

import (
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
)

const DataTransferProtocol = "/fil/storage/transfer/1.0.0"

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

type TransportState int64

const (
	TransportUnknown TransportState = iota
	Transporting
	TransportCompleted
	TransportFailed
)

type TransportInfo struct {
	ProposalCID    cid.Cid
	OutputFile     string
	Transfer       market.Transfer
	NBytesReceived int64
}

// TransportEvent is fired as a transfer progresses
type TransportEvent struct {
	NBytesReceived int64
	Error          error
}
