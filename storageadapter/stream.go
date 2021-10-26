package storageadapter

import (
	"context"
	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	types2 "github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus/pkg/wallet"
	"golang.org/x/xerrors"
	"os"
)

var _ network.StorageReceiver = (*StorageDealStream)(nil)

type StorageDealStream struct {
	conns       *connmanager.ConnManager
	storedAsk   StorageAsk
	spn         StorageProviderNode
	deals       StorageDealStore
	net         network.StorageMarketNetwork
	fs          filestore.FileStore
	dealProcess StorageDealProcess
}

//********* Network *******//
func (storageDealStream *StorageDealStream) HandleAskStream(s network.StorageAskStream) {
	defer s.Close()
	ar, err := s.ReadAskRequest()
	if err != nil {
		log.Errorf("failed to read AskRequest from incoming stream: %s", err)
		return
	}

	ask, err := storageDealStream.storedAsk.GetAsk(ar.Miner)
	if err != nil {
		if xerrors.Is(err, RecordNotFound) {
			log.Warnf(" receive ask for miner with address %s", ar.Miner)
		} else {
			//write error?
		}
	}

	resp := network.AskResponse{
		Ask: ask,
	}

	if err := s.WriteAskResponse(resp, storageDealStream.spn.Sign); err != nil {
		log.Errorf("failed to write ask response: %s", err)
		return
	}
}

func (storageDealStream *StorageDealStream) HandleDealStream(s network.StorageDealStream) {
	ctx := context.TODO()
	defer s.Close()
	proposal, err := s.ReadDealProposal()
	if err != nil {
		log.Errorf("failed to read proposal message: %w", err)
		return
	}

	proposalNd, err := cborutil.AsIpld(proposal.DealProposal)
	if err != nil {
		log.Errorf("unable to market deal proposal %w", err)
		return
	}

	// Check if we are already tracking this deal
	md, err := storageDealStream.deals.GetDeal(proposalNd.Cid())
	if err == nil {
		// We are already tracking this deal, for some reason it was re-proposed, perhaps because of a client restart
		// this is ok, just send a response back.
		err := storageDealStream.resendProposalResponse(s, md)
		if err != nil {
			log.Errorf("unable to market deal proposal %w", err)
			return
		}
	}

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

	deal := &storagemarket.MinerDeal{
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

	err = storageDealStream.deals.SaveDeal(deal)
	if err != nil {
		log.Errorf("save miner deal to database %w", err)
		return
	}
	err = storageDealStream.conns.AddStream(proposalNd.Cid(), s)
	if err != nil {
		log.Errorf("add stream to connection %s %w", proposalNd.Cid(), err)
	}

	err = storageDealStream.dealProcess.AcceptDeal(ctx, deal)
	if err != nil {
		log.Errorf("fail accept deal %s %w", proposalNd.Cid(), err)
	}
}

func (storageDealStream *StorageDealStream) HandleDealStatusStream(s network.DealStatusStream) {
	ctx := context.TODO()
	defer s.Close()
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

	signature, err := storageDealStream.spn.Sign(ctx, &types2.SignInfo{
		Data: dealState,
		Type: wallet.MTUnknown,
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

	if err := s.WriteDealStatusResponse(response, storageDealStream.spn.Sign); err != nil {
		log.Warnf("failed to write deal status response: %s", err)
		return
	}
}

func (storageDealStream *StorageDealStream) resendProposalResponse(s network.StorageDealStream, md *storagemarket.MinerDeal) error {
	resp := &network.Response{State: md.State, Message: md.Message, Proposal: md.ProposalCid}
	sig, err := storageDealStream.spn.Sign(context.TODO(), &types2.SignInfo{
		Data: resp,
		Type: wallet.MTUnknown, // todo
		Addr: address.Address{},
	})
	if err != nil {
		return xerrors.Errorf("failed to sign response message: %w", err)
	}
	// todo use resign
	err = s.WriteDealResponse(network.SignedResponse{Response: *resp, Signature: sig}, storageDealStream.spn.Sign)

	if closeErr := s.Close(); closeErr != nil {
		log.Warnf("closing connection: %v", err)
	}

	return err
}

func (storageDealStream *StorageDealStream) processDealStatusRequest(ctx context.Context, request *network.DealStatusRequest) (*storagemarket.ProviderDealState, address.Address, error) {
	// fetch deal state
	md, err := storageDealStream.deals.GetDeal(request.Proposal)
	if err != nil {
		log.Errorf("proposal doesn't exist in state store: %s", err)
		return nil, address.Undef, xerrors.Errorf("no such proposal")
	}

	// verify query signature
	buf, err := cborutil.Dump(&request.Proposal)
	if err != nil {
		log.Errorf("failed to serialize status request: %s", err)
		return nil, address.Undef, xerrors.Errorf("internal error")
	}

	tok, _, err := storageDealStream.spn.GetChainHead(ctx)
	if err != nil {
		log.Errorf("failed to get chain head: %s", err)
		return nil, address.Undef, xerrors.Errorf("internal error")
	}

	err = providerutils.VerifySignature(ctx, request.Signature, md.ClientDealProposal.Proposal.Client, buf, tok, storageDealStream.spn.VerifySignature)
	if err != nil {
		log.Errorf("invalid deal status request signature: %s", err)
		return nil, address.Undef, xerrors.Errorf("internal error")
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
