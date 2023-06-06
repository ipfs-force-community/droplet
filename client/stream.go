package client

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/libp2p/go-libp2p/core/host"
)

type ClientStream struct {
	h    host.Host
	node storagemarket.StorageClientNode
	net  network.StorageMarketNetwork
}

func NewClientStream(h host.Host, node storagemarket.StorageClientNode) *ClientStream {
	// go-fil-markets protocol retries:
	// 1s, 5s, 25s, 2m5s, 5m x 11 ~= 1 hour
	marketsRetryParams := network.RetryParameters(time.Second, 5*time.Minute, 15, 5)
	net := network.NewFromLibp2pHost(h, marketsRetryParams)

	return &ClientStream{h: h, node: node, net: net}
}

func (cs *ClientStream) GetDealState(ctx context.Context,
	deal *client.ClientOfflineDeal,
	minerInfo types.MinerInfo,
) (*storagemarket.ProviderDealState, error) {
	if len(minerInfo.Multiaddrs) > 0 {
		multiaddr, err := utils.ConvertMultiaddr(minerInfo.Multiaddrs)
		if err == nil {
			cs.net.AddAddrs(*minerInfo.PeerId, multiaddr)
		}
	}
	s, err := cs.net.NewDealStatusStream(ctx, *minerInfo.PeerId)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream to miner: %w", err)
	}
	defer s.Close() //nolint

	buf, err := cborutil.Dump(deal.ProposalCID)
	if err != nil {
		return nil, fmt.Errorf("failed serialize deal status request: %w", err)
	}
	signature, err := cs.node.SignBytes(ctx, deal.Proposal.Client, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to sign deal status request: %w", err)
	}

	if err := s.WriteDealStatusRequest(network.DealStatusRequest{Proposal: deal.ProposalCID, Signature: *signature}); err != nil {
		return nil, fmt.Errorf("failed to send deal status request: %w", err)
	}

	resp, origBytes, err := s.ReadDealStatusResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read deal status response: %w", err)
	}

	valid, err := cs.verifyStatusResponseSignature(ctx, minerInfo.Worker, resp, origBytes)
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, fmt.Errorf("invalid deal status response signature")
	}

	return &resp.DealState, nil
}

func (cs *ClientStream) verifyStatusResponseSignature(ctx context.Context,
	miner address.Address,
	response network.DealStatusResponse,
	origBytes []byte,
) (bool, error) {
	tok, _, err := cs.node.GetChainHead(ctx)
	if err != nil {
		return false, fmt.Errorf("getting chain head: %w", err)
	}

	valid, err := cs.node.VerifySignature(ctx, response.Signature, miner, origBytes, tok)
	if err != nil {
		return false, fmt.Errorf("validating signature: %w", err)
	}

	return valid, nil
}
