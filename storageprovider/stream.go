package storageprovider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/google/uuid"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"

	"github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/utils"
	network2 "github.com/libp2p/go-libp2p/core/network"

	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
)

var _ network.StorageReceiver = (*StorageDealStream)(nil)

type StorageDealStream struct {
	conns          *connmanager.ConnManager
	storedAsk      IStorageAsk
	spn            StorageProviderNode
	deals          repo.StorageDealRepo
	net            network.StorageMarketNetwork
	tf             config.TransferFileStoreConfigFunc
	dealProcess    StorageDealHandler
	mixMsgClient   clients.IMixMessage
	eventPublisher *EventPublishAdapter
}

// NewStorageReceiver returns a new StorageReceiver implements functions for receiving incoming data on storage protocols
func NewStorageDealStream(
	conns *connmanager.ConnManager,
	storedAsk IStorageAsk,
	spn StorageProviderNode,
	deals repo.StorageDealRepo,
	net network.StorageMarketNetwork,
	tf config.TransferFileStoreConfigFunc,
	dealProcess StorageDealHandler,
	mixMsgClient clients.IMixMessage,
	pubsub *EventPublishAdapter,
) (*StorageDealStream, error) {
	return &StorageDealStream{
		conns:          conns,
		storedAsk:      storedAsk,
		spn:            spn,
		deals:          deals,
		net:            net,
		tf:             tf,
		dealProcess:    dealProcess,
		mixMsgClient:   mixMsgClient,
		eventPublisher: pubsub,
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

	// if err := s.WriteAskResponse(resp, storageDealStream.spn.SignWithGivenMiner(ar.Miner)); err != nil {
	if err := s.WriteAskResponse(resp, nil); err != nil {
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
		fs, err := storageDealStream.tf(proposal.DealProposal.Proposal.Provider)
		if err != nil {
			log.Errorf("failed to create temp file store for provider %s: %w", proposal.DealProposal.Proposal.Provider.String(), err)
			return
		}
		tmp, err := fs.CreateTemp()
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
		ID:                 uuid.New(),
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

	storageDealStream.eventPublisher.Publish(storagemarket.ProviderEventOpen, deal)

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
		Type: vTypes.MTProviderDealState,
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

	if err := s.WriteDealStatusResponse(response, nil); err != nil {
		log.Warnf("failed to write deal status response: %s", err)
		return
	}
}

func (storageDealStream *StorageDealStream) resendProposalResponse(s network.StorageDealStream, md *types.MinerDeal) error {
	resp := &network.Response{State: md.State, Message: md.Message, Proposal: md.ProposalCid}
	sig, err := storageDealStream.spn.Sign(context.TODO(), &types.SignInfo{
		Data: resp,
		Type: vTypes.MTNetWorkResponse,
		Addr: md.Proposal.Provider,
	})
	if err != nil {
		storageDealStream.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, md)
		return fmt.Errorf("failed to sign response message: %w", err)
	}

	return s.WriteDealResponse(network.SignedResponse{Response: *resp, Signature: sig}, nil)
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
		storageDealStream.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, md)
		log.Errorf("failed to get chain head: %s", err)
		return nil, address.Undef, fmt.Errorf("internal error")
	}

	err = providerutils.VerifySignature(ctx, request.Signature, md.ClientDealProposal.Proposal.Client, buf, tok, storageDealStream.spn.VerifySignature)
	if err != nil {
		storageDealStream.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, md)
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

// The time limit to read a message from the client when the client opens a stream
const providerReadDeadline = 10 * time.Second

// The time limit to write a response to the client
const providerWriteDeadline = 10 * time.Second

// boost protocol

func (storageDealStream *StorageDealStream) HandleNewDealStream(s network2.Stream) {
	start := time.Now()
	log := log.With("client-peer", s.Conn().RemotePeer())

	defer func() {
		err := s.Close()
		if err != nil {
			log.Infow("closing stream", "err", err)
		}
		log.Debugw("handled deal proposal request", "duration", time.Since(start).String())
	}()

	// Set a deadline on reading from the stream so it doesn't hang
	_ = s.SetReadDeadline(time.Now().Add(providerReadDeadline))

	// Read the deal proposal from the stream
	var proposal types2.DealParams
	err := proposal.UnmarshalCBOR(s)
	_ = s.SetReadDeadline(time.Time{}) // Clear read deadline so conn doesn't get closed
	if err != nil {
		log.Warnw("reading storage deal proposal from stream", "err", err)
		return
	}

	log = log.With("id", proposal.DealUUID)
	log.Infow("received deal proposal")

	ctx := context.Background()

	writeNewDealResponse := func(s network2.Stream, accepted bool, reason string) {
		if len(reason) != 0 {
			log.Warn(reason)
		}
		// Write the response to the client
		err := cborutil.WriteCborRPC(s, &types2.DealResponse{Accepted: accepted, Message: reason})
		if err != nil {
			log.Warnw("writing deal response", "err", err)
		}
	}

	if !proposal.IsOffline {
		writeNewDealResponse(s, false, "not support online deal")
		return
	}

	// Check if we are already tracking this deal
	_, err = storageDealStream.deals.GetDealByUUID(ctx, proposal.DealUUID)
	if err == nil {
		writeNewDealResponse(s, false, "same deal exists")
		return
	}

	proposalNd, err := cborutil.AsIpld(&proposal.ClientDealProposal)
	if err != nil {
		writeNewDealResponse(s, false, fmt.Sprintf("deal proposal cbor failed: %v", err))
		return
	}

	deal := &types.MinerDeal{
		ID:                 uuid.New(),
		Client:             s.Conn().RemotePeer(),
		Miner:              storageDealStream.net.ID(),
		ClientDealProposal: proposal.ClientDealProposal,
		ProposalCid:        proposalNd.Cid(),
		State:              storagemarket.StorageDealUnknown,
		PieceStatus:        types.Undefine,
		Ref: &storagemarket.DataRef{
			TransferType: proposal.Transfer.Type,
			Root:         proposal.DealDataRoot,
			PieceCid:     &proposal.ClientDealProposal.Proposal.PieceCID,
			PieceSize:    proposal.ClientDealProposal.Proposal.PieceSize.Unpadded(),
			RawBlockSize: proposal.Transfer.Size,
		},
		FastRetrieval: true,
		CreationTime:  curTime(),
	}
	err = storageDealStream.deals.SaveDeal(ctx, deal)
	if err != nil {
		log.Errorf("save miner deal to database %v", err)
		return
	}

	var reason string
	accepted := true
	deal.State = storagemarket.StorageDealWaitingForData

	err = storageDealStream.dealProcess.AcceptNewDeal(ctx, deal)
	if err != nil {
		reason = err.Error()
		deal.Message = reason
		deal.State = storagemarket.StorageDealRejecting
		accepted = false
	}

	go func() {
		if err := storageDealStream.deals.SaveDeal(ctx, deal); err != nil {
			log.Errorf("save deal failed: %v", err)
		}
	}()

	// Set a deadline on writing to the stream so it doesn't hang
	_ = s.SetWriteDeadline(time.Now().Add(providerWriteDeadline))
	defer s.SetWriteDeadline(time.Time{}) // nolint

	writeNewDealResponse(s, accepted, reason)
}
