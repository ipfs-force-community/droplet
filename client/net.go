package client

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/venus-market/v2/types"
	"github.com/google/uuid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"golang.org/x/xerrors"

	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	vtypes "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
)

var dealStreamLog = logging.Logger("deal-client")

// DealStreamOption is an option for configuring the libp2p storage deal client
type DealStreamOption func(*DealStream)

// RetryParameters changes the default parameters around connection reopening
func RetryParameters(minDuration time.Duration, maxDuration time.Duration, attempts float64, backoffFactor float64) DealStreamOption {
	return func(c *DealStream) {
		c.retryStream.SetOptions(shared.RetryParameters(minDuration, maxDuration, attempts, backoffFactor))
	}
}

// DealStream sends deal proposals over libp2p
type DealStream struct {
	retryStream *shared.RetryStream
	api         v1.FullNode
}

// SendDealProposal sends a deal proposal over a libp2p stream to the peer
func (c *DealStream) SendDealProposal(ctx context.Context, id peer.ID, params *market.DealParams) (*types.DealResponse, error) {
	dealStreamLog.Debugw("send deal proposal", "id", params.DealUUID, "provider-peer", id)

	// Create a libp2p stream to the provider
	s, err := c.retryStream.OpenStream(ctx, id, []protocol.ID{types.DealProtocolID})
	if err != nil {
		return nil, err
	}

	defer s.Close() // nolint

	// Set a deadline on writing to the stream so it doesn't hang
	_ = s.SetWriteDeadline(time.Now().Add(types.ClientWriteDeadline))
	defer s.SetWriteDeadline(time.Time{}) // nolint

	// Write the deal proposal to the stream
	if err = cborutil.WriteCborRPC(s, params); err != nil {
		return nil, xerrors.Errorf("sending deal proposal: %w", err)
	}

	// Set a deadline on reading from the stream so it doesn't hang
	_ = s.SetReadDeadline(time.Now().Add(types.ClientReadDeadline))
	defer s.SetReadDeadline(time.Time{}) // nolint

	// Read the response from the stream
	var resp types.DealResponse
	if err := resp.UnmarshalCBOR(s); err != nil {
		return nil, xerrors.Errorf("reading proposal response: %w", err)
	}

	dealStreamLog.Debugw("received deal proposal response", "id", params.DealUUID, "accepted", resp.Accepted, "reason", resp.Message)

	return &resp, nil
}

func (c *DealStream) SendDealStatusRequest(ctx context.Context, addr address.Address, id peer.ID, dealUUID uuid.UUID) (*types.DealStatusResponse, error) {
	dealStreamLog.Debugw("send deal status req", "deal-uuid", dealUUID, "id", id)

	uuidBytes, err := dealUUID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("getting uuid bytes: %w", err)
	}

	sig, err := c.api.WalletSign(ctx, addr, uuidBytes, vtypes.MsgMeta{Type: vtypes.MTDealProposal})
	if err != nil {
		return nil, fmt.Errorf("signing uuid bytes: %w", err)
	}

	// Create a libp2p stream to the provider
	s, err := c.retryStream.OpenStream(ctx, id, []protocol.ID{types.DealStatusV12ProtocolID})
	if err != nil {
		return nil, err
	}

	defer s.Close() // nolint

	// Set a deadline on writing to the stream so it doesn't hang
	_ = s.SetWriteDeadline(time.Now().Add(types.ClientWriteDeadline))
	defer s.SetWriteDeadline(time.Time{}) // nolint

	// Write the deal status request to the stream
	req := types.DealStatusRequest{DealUUID: dealUUID, Signature: *sig}
	if err = cborutil.WriteCborRPC(s, &req); err != nil {
		return nil, fmt.Errorf("sending deal status req: %w", err)
	}

	// Set a deadline on reading from the stream so it doesn't hang
	_ = s.SetReadDeadline(time.Now().Add(types.ClientReadDeadline))
	defer s.SetReadDeadline(time.Time{}) // nolint

	// Read the response from the stream
	var resp types.DealStatusResponse
	if err := resp.UnmarshalCBOR(s); err != nil {
		return nil, fmt.Errorf("reading deal status response: %w", err)
	}

	dealStreamLog.Debugw("received deal status response", "id", resp.DealUUID, "status", resp.DealStatus)

	return &resp, nil
}

func newDealStream(h host.Host, api v1.FullNode, options ...DealStreamOption) *DealStream {
	c := &DealStream{
		retryStream: shared.NewRetryStream(h),
		api:         api,
	}
	for _, option := range options {
		option(c)
	}
	return c
}
