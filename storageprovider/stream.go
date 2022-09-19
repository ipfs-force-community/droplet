package storageprovider

import (
	"context"
	"fmt"
	"os"

	"github.com/filecoin-project/venus-market/v2/api/clients"
	"github.com/filecoin-project/venus-market/v2/utils"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"

	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

var _ network.StorageReceiver = (*StorageDealStream)(nil)

type StorageDealStream struct {
	conns        *connmanager.ConnManager
	storedAsk    IStorageAsk
	spn          StorageProviderNode
	deals        repo.StorageDealRepo
	net          network.StorageMarketNetwork
	fs           filestore.FileStore
	dealProcess  StorageDealHandler
	mixMsgClient clients.IMixMessage
}

// NewStorageReceiver returns a new StorageReceiver implements functions for receiving incoming data on storage protocols
func NewStorageDealStream(
	conns *connmanager.ConnManager,
	storedAsk IStorageAsk,
	spn StorageProviderNode,
	deals repo.StorageDealRepo,
	net network.StorageMarketNetwork,
	fs filestore.FileStore,
	dealProcess StorageDealHandler,
	mixMsgClient clients.IMixMessage,
) (network.StorageReceiver, error) {

	return &StorageDealStream{
		conns:        conns,
		storedAsk:    storedAsk,
		spn:          spn,
		deals:        deals,
		net:          net,
		fs:           fs,
		dealProcess:  dealProcess,
		mixMsgClient: mixMsgClient,
	}, nil
}

/*
HandleAskStream is called by the network implementation whenever a new message is received on the ask protocol

A Provider handling a `AskRequest` does the following:

1. Reads the current signed storage ask from storage

2. Wraps the signed ask in an AskResponse and writes it on the StorageAskStream

The connection is kept open only as long as the request-response exchange.
*/
func (storageDealStream *StorageDealStream) HandleAskStream(s network.StorageAskStream) {
	defer func() {
		if err := s.Close(); err != nil {
			log.Errorf("unable to close err %v", err)
		}
	}()
	ar, err := s.ReadAskRequest()
	if err != nil {
		log.Errorf("failed to read AskRequest from incoming stream: %s", err)
		return
	}

	ask, err := storageDealStream.storedAsk.GetAsk(context.TODO(), ar.Miner)
	if err != nil {
		log.Errorf("failed to get ask for [%s]: %s", ar.Miner, err)
		return
	}

	resp := network.AskResponse{
		Ask: ask.ToChainAsk(),
	}

	if err := s.WriteAskResponse(resp, storageDealStream.spn.SignWithGivenMiner(ar.Miner)); err != nil {
		log.Errorf("failed to write ask response: %s", err)
		return
	}
}

func (storageDealStream *StorageDealStream) HandleDealStream(s network.StorageDealStream) {
	ctx := context.TODO()
	defer func() {
		if closeErr := s.Close(); closeErr != nil {
			log.Warnf("closing connection: %v", closeErr)
		}
	}()

	// 1. Calculates the CID for the received ClientDealProposal.
	proposal, err := s.ReadDealProposal()
	if err != nil {
		log.Errorf("failed to read proposal message: %w", err)
		return
	}

	proposalNd, err := cborutil.AsIpld(proposal.DealProposal)
	if err != nil {
		log.Errorf("deal proposal cbor failed: %w", err)
		return
	}

	// Check if we are already tracking this deal
	md, err := storageDealStream.deals.GetDeal(ctx, proposalNd.Cid())
	if err == nil {
		// We are already tracking this deal, for some reason it was re-proposed, perhaps because of a client restart
		// this is ok, just send a response back.
		err = storageDealStream.resendProposalResponse(s, md)
		if err != nil {
			log.Errorf("unable to market deal proposal %w", err)
		}
		return
	}

	// 2. Constructs a MinerDeal to track the state of this deal.
	var path string
	// create an empty CARv2 file at a temp location that Graphysnc will write the incoming blocks to via a CARv2 ReadWrite blockstore wrapper.
	if proposal.Piece.TransferType != storagemarket.TTManual {
		tmp, err := storageDealStream.fs.CreateTemp()
		if err != nil {
			log.Errorf("failed to create an empty temp CARv2 file: %w", err)
			return
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(string(tmp.OsPath()))
			log.Errorf("failed to close temp file: %w", err)
			return
		}
		path = string(tmp.OsPath())
	}

	deal := &types.MinerDeal{
		Client:             s.RemotePeer(),
		Miner:              storageDealStream.net.ID(),
		ClientDealProposal: *proposal.DealProposal,
		ProposalCid:        proposalNd.Cid(),
		State:              storagemarket.StorageDealUnknown,
		Ref:                proposal.Piece,
		FastRetrieval:      proposal.FastRetrieval,
		CreationTime:       curTime(),
		InboundCAR:         path,
	}
	err = storageDealStream.deals.SaveDeal(ctx, deal)
	if err != nil {
		log.Errorf("save miner deal to database %w", err)
		return
	}

	err = storageDealStream.conns.AddStream(proposalNd.Cid(), s)
	if err != nil {
		log.Errorf("add stream to connection %s %w", proposalNd.Cid(), err)
		return
	}

	err = storageDealStream.dealProcess.AcceptDeal(ctx, deal)
	if err != nil {
		log.Errorf("fail accept deal %s %w", proposalNd.Cid(), err)
	}
}

/*
HandleDealStatusStream is called by the network implementation whenever a new message is received on the deal status protocol

A Provider handling a `DealStatuRequest` does the following:

1. Lots the deal state from the StorageDealStore

2. Verifies the signature on the DealStatusRequest matches the Client for this deal

3. Constructs a ProviderDealState from the deal state

4. Signs the ProviderDealState with its private key

5. Writes a DealStatusResponse with the ProviderDealState and signature onto the DealStatusStream

The connection is kept open only as long as the request-response exchange.
*/
func (storageDealStream *StorageDealStream) HandleDealStatusStream(s network.DealStatusStream) {
	ctx := context.TODO()
	defer func() {
		if closeErr := s.Close(); closeErr != nil {
			log.Warnf("closing connection: %v", closeErr)
		}
	}()

	// 1. Lots the deal state from the StorageDealStore
	request, err := s.ReadDealStatusRequest()
	if err != nil {
		log.Errorf("failed to read DealStatusRequest from incoming stream: %s", err)
		return
	}

	dealState, mAddr, err := storageDealStream.processDealStatusRequest(ctx, &request)
	if err != nil {
		log.Errorf("failed to process deal status request: %s", err)
		dealState = &storagemarket.ProviderDealState{
			State:   storagemarket.StorageDealError,
			Message: err.Error(),
		}
	}

	signature, err := storageDealStream.spn.Sign(ctx, &types.SignInfo{
		Data: dealState,
		Type: vTypes.MTUnknown,
		Addr: mAddr,
	})
	if err != nil {
		log.Errorf("failed to sign deal status response: %s", err)
		return
	}

	response := network.DealStatusResponse{
		DealState: *dealState,
		Signature: *signature,
	}

	if err := s.WriteDealStatusResponse(response, storageDealStream.spn.SignWithGivenMiner(mAddr)); err != nil {
		log.Warnf("failed to write deal status response: %s", err)
		return
	}
}

func (storageDealStream *StorageDealStream) resendProposalResponse(s network.StorageDealStream, md *types.MinerDeal) error {
	resp := &network.Response{State: md.State, Message: md.Message, Proposal: md.ProposalCid}
	sig, err := storageDealStream.spn.Sign(context.TODO(), &types.SignInfo{
		Data: resp,
		Type: vTypes.MTUnknown,
		Addr: md.Proposal.Provider,
	})
	if err != nil {
		return fmt.Errorf("failed to sign response message: %w", err)
	}

	return s.WriteDealResponse(network.SignedResponse{Response: *resp, Signature: sig}, storageDealStream.spn.SignWithGivenMiner(md.Proposal.Provider))
}

func (storageDealStream *StorageDealStream) processDealStatusRequest(ctx context.Context, request *network.DealStatusRequest) (*storagemarket.ProviderDealState, address.Address, error) {
	// fetch deal state
	md, err := storageDealStream.deals.GetDeal(ctx, request.Proposal)
	if err != nil {
		log.Errorf("proposal doesn't exist in state store: %s", err)
		return nil, address.Undef, fmt.Errorf("no such proposal")
	}

	// verify query signature
	buf, err := cborutil.Dump(&request.Proposal)
	if err != nil {
		log.Errorf("failed to serialize status request: %s", err)
		return nil, address.Undef, fmt.Errorf("internal error")
	}

	tok, _, err := storageDealStream.spn.GetChainHead(ctx)
	if err != nil {
		log.Errorf("failed to get chain head: %s", err)
		return nil, address.Undef, fmt.Errorf("internal error")
	}

	err = providerutils.VerifySignature(ctx, request.Signature, md.ClientDealProposal.Proposal.Client, buf, tok, storageDealStream.spn.VerifySignature)
	if err != nil {
		log.Errorf("invalid deal status request signature: %s", err)
		return nil, address.Undef, fmt.Errorf("internal error")
	}

	if md.AddFundsCid != nil && md.AddFundsCid.Prefix() == utils.MidPrefix {
		md.AddFundsCid, err = storageDealStream.mixMsgClient.GetMessageChainCid(ctx, *md.AddFundsCid)
		if err != nil {
			log.Errorf("unbale to get add funds message cid: %s", err)
			return nil, address.Undef, fmt.Errorf("internal error")
		}
	}

	if md.PublishCid != nil && md.PublishCid.Prefix() == utils.MidPrefix {
		md.PublishCid, err = storageDealStream.mixMsgClient.GetMessageChainCid(ctx, *md.PublishCid)
		if err != nil {
			log.Errorf("unbale to get publish message cid: %s", err)
			return nil, address.Undef, fmt.Errorf("internal error")
		}
	}

	return &storagemarket.ProviderDealState{
		State:         md.State,
		Message:       md.Message,
		Proposal:      &md.Proposal,
		ProposalCid:   &md.ProposalCid,
		AddFundsCid:   md.AddFundsCid,
		PublishCid:    md.PublishCid,
		DealID:        md.DealID,
		FastRetrieval: md.FastRetrieval,
	}, md.ClientDealProposal.Proposal.Provider, nil
}
