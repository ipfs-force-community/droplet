package storageprovider

import (
	"context"
	"fmt"
	"time"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/venus/pkg/crypto"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	vtypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus-market/v2/api/clients"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types2 "github.com/filecoin-project/venus-market/v2/types"
)

var boostStreamLog = logging.Logger("boost-stream")

type BoostStorageDealStream struct {
	host          host.Host
	deals         repo.StorageDealRepo
	net           network.StorageMarketNetwork
	dealProcess   StorageDealHandler
	dealTransport *DealTransport
	mixMsgClient  clients.IMixMessage
	full          v1.FullNode
	fs            filestore.FileStore
}

func NewBoostStorageDealStream(host host.Host,
	repo repo.Repo,
	net network.StorageMarketNetwork,
	dealProcess StorageDealHandler,
	dealTransport *DealTransport,
	mixMsgClient clients.IMixMessage,
	full v1.FullNode,
	fs filestore.FileStore) *BoostStorageDealStream {

	return &BoostStorageDealStream{
		host:          host,
		deals:         repo.StorageDealRepo(),
		net:           net,
		dealProcess:   dealProcess,
		dealTransport: dealTransport,
		mixMsgClient:  mixMsgClient,
		full:          full,
		fs:            fs,
	}
}

func (stream *BoostStorageDealStream) Start() {
	stream.host.SetStreamHandler(types2.DealProtocolID, stream.HandleNewDealStream)
	stream.host.SetStreamHandler(types2.DealStatusV12ProtocolID, stream.HandleNewDealStatusStream)
}

func (stream *BoostStorageDealStream) Stop() {
	stream.host.RemoveStreamHandler(types2.DealProtocolID)
	stream.host.RemoveStreamHandler(types2.DealStatusV12ProtocolID)
}

// Called when the client opens a libp2p stream with a new deal proposal
func (stream *BoostStorageDealStream) HandleNewDealStream(s libp2pnetwork.Stream) {
	defer s.Close() //nolint:errcheck

	ctx := context.Background()

	// Set a deadline on reading from the stream so it doesn't hang
	_ = s.SetReadDeadline(time.Now().Add(types2.ProviderReadDeadline))
	defer s.SetReadDeadline(time.Time{}) // nolint

	// Read the deal proposal from the stream
	var proposal types.DealParams
	err := proposal.UnmarshalCBOR(s)
	if err != nil {
		boostStreamLog.Warnw("reading storage deal proposal from stream", "err", err)
		return
	}

	proposalNd, err := cborutil.AsIpld(&proposal.ClientDealProposal)
	if err != nil {
		boostStreamLog.Errorf("deal proposal cbor failed: %w", err)
		return
	}
	proposalCID := proposalNd.Cid()

	log := boostStreamLog.With("proposal id", proposalCID, "uuid", proposal.DealUUID)

	log.Infow("received deal proposal", "client-peer", s.Conn().RemotePeer())

	sendResp := func(res types.ProviderDealRejectionInfo) error {
		// Set a deadline on writing to the stream so it doesn't hang
		_ = s.SetWriteDeadline(time.Now().Add(types2.ProviderWriteDeadline))
		defer s.SetWriteDeadline(time.Time{}) // nolint

		// Write the response to the client
		log.Infow("send deal proposal response", "accepted", res.Accepted, "msg", res.Reason)
		err = cborutil.WriteCborRPC(s, &types2.DealResponse{Accepted: res.Accepted, Message: res.Reason})
		return err
	}

	resp := types.ProviderDealRejectionInfo{Accepted: true}

	// Check if we are already tracking this deal
	md, err := stream.deals.GetDeal(ctx, proposalCID)
	if err == nil {
		// We are already tracking this deal, for some reason it was re-proposed, perhaps because of a client restart
		// this is ok, just send a response back.
		if md.State != storagemarket.StorageDealWaitingForData {
			resp.Accepted = false
			resp.Reason = fmt.Sprintf("provider returned unexpected state %d for proposal %s, with message: %s", md.State, proposalCID, md.Message)
		}
		if err := sendResp(resp); err != nil {
			log.Errorf("writing deal response", "err", err)
		}
		return
	}

	deal := &types.MinerDeal{
		Client:             s.Conn().RemotePeer(),
		Miner:              stream.net.ID(),
		ClientDealProposal: proposal.ClientDealProposal,
		ProposalCid:        proposalNd.Cid(),
		State:              storagemarket.StorageDealUnknown,
		Ref: &types.DataRef{
			TransferType: proposal.Transfer.Type,
			Root:         proposal.DealDataRoot,
			Params:       proposal.Transfer.Params,
			State:        int64(types2.TransportUnknown),
			DealUUID:     vtypes.UUID(proposal.DealUUID),
			PieceCid:     &proposal.ClientDealProposal.Proposal.PieceCID,
			PieceSize:    proposal.ClientDealProposal.Proposal.PieceSize.Unpadded(),
			RawBlockSize: proposal.Transfer.Size,
		},
		// todo: fill FastRetrieval
		//FastRetrieval:      ,
		CreationTime: curTime(),
	}
	err = stream.deals.SaveDeal(ctx, deal)
	if err != nil {
		log.Errorf("save miner deal to database %w", err)
		return
	}

	err = stream.dealProcess.VerifyDeal(ctx, deal)
	if err != nil {
		resp.Accepted = false
		resp.Reason = err.Error()
	}

	// todo: release reserved funds, when failed to send response ?
	err = sendResp(resp)
	if err != nil {
		errMsg := fmt.Sprintf("id %v, proposal id %v, writing deal response err: %v", proposal.DealUUID, proposalCID, err)
		if err := stream.dealProcess.HandleError(ctx, deal, err); err != nil {
			errMsg += fmt.Sprintf(", call handleError failed %v", err)
		}
		boostStreamLog.Error(errMsg)
		return
	}
	deal.State = storagemarket.StorageDealWaitingForData
	if err := stream.deals.SaveDeal(ctx, deal); err != nil {
		log.Errorf("save deal state %v failed %v", storagemarket.StorageDealWaitingForData, err)
		return
	}
	ti := &types2.TransportInfo{
		ProposalCID: proposalCID,
		Transfer:    proposal.Transfer,
	}
	go func() {
		if err := stream.dealTransport.TransportData(ctx, ti, deal); err != nil {
			log.Warnf("failed to transport deal %v", err)
		}
	}()
}

func (stream *BoostStorageDealStream) HandleNewDealStatusStream(s libp2pnetwork.Stream) {
	defer s.Close() //nolint:errcheck

	_ = s.SetReadDeadline(time.Now().Add(types2.ProviderReadDeadline))
	defer s.SetReadDeadline(time.Time{}) // nolint

	var req types2.DealStatusRequest
	err := req.UnmarshalCBOR(s)
	if err != nil {
		boostStreamLog.Warnw("reading deal status request from stream", "err", err)
		return
	}
	boostStreamLog.Debugw("received deal status request", "id", req.DealUUID, "client-peer", s.Conn().RemotePeer())

	resp := stream.getDealStatus(req)

	// Set a deadline on writing to the stream so it doesn't hang
	_ = s.SetWriteDeadline(time.Now().Add(types2.ProviderWriteDeadline))
	defer s.SetWriteDeadline(time.Time{}) // nolint

	if err := cborutil.WriteCborRPC(s, &resp); err != nil {
		boostStreamLog.Errorw("failed to write deal status response", "err", err)
		return
	}
}

func (stream *BoostStorageDealStream) getDealStatus(req types2.DealStatusRequest) types2.DealStatusResponse {
	ctx := context.Background()
	errResp := func(err string) types2.DealStatusResponse {
		return types2.DealStatusResponse{DealUUID: req.DealUUID, Error: err}
	}

	pds, err := stream.deals.GetDealByDealUUID(ctx, vtypes.UUID(req.DealUUID))
	if err != nil && xerrors.Is(err, repo.ErrNotFound) {
		return errResp(fmt.Sprintf("no storage deal found with deal UUID %s", req.DealUUID))
	}

	if err != nil {
		boostStreamLog.Errorw("failed to fetch deal status", "err", err)
		return errResp("failed to fetch deal status")
	}

	// verify request signature
	uuidBytes, err := req.DealUUID.MarshalBinary()
	if err != nil {
		boostStreamLog.Errorw("failed to serialize request deal UUID", "err", err)
		return errResp("failed to serialize request deal UUID")
	}

	clientAddr := pds.ClientDealProposal.Proposal.Client
	addr, err := stream.full.StateAccountKey(ctx, clientAddr, vtypes.EmptyTSK)
	if err != nil {
		boostStreamLog.Errorw("failed to get account key for client addr", "client", clientAddr.String(), "err", err)
		msg := fmt.Sprintf("failed to get account key for client addr %s", clientAddr.String())
		return errResp(msg)
	}

	err = crypto.Verify(&req.Signature, addr, uuidBytes)
	if err != nil {
		boostStreamLog.Warnw("signature verification failed", "err", err)
		return errResp("signature verification failed")
	}

	var bts uint64
	f, err := stream.fs.Open(pds.PiecePath)
	if err != nil {
		boostStreamLog.Warnf("deal %v open %s(piecepath) failed %v", pds.ProposalCid, pds.PiecePath, err)
	} else {
		bts = uint64(f.Size())
		_ = f.Close()
	}

	return types2.DealStatusResponse{
		DealUUID: req.DealUUID,
		DealStatus: &types2.DealStatus{
			//Error:             pds.Err,
			//Status:            pds.Checkpoint.String(),
			Proposal:          pds.ClientDealProposal.Proposal,
			SignedProposalCid: pds.ProposalCid,
			PublishCid:        pds.PublishCid,
			ChainDealID:       pds.DealID,
		},
		//IsOffline:      ,
		TransferSize:   pds.PayloadSize,
		NBytesReceived: bts,
	}
}
