package retrievalprovider

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	peer "github.com/libp2p/go-libp2p-core/peer"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/migrations"
)

var allSelectorBytes []byte

var askTimeout = 5 * time.Second

func init() {
	buf := new(bytes.Buffer)
	_ = dagcbor.Encode(selectorparse.CommonSelector_ExploreAllRecursively, buf)
	allSelectorBytes = buf.Bytes()
}

// ProviderRequestValidator validates incoming requests for the Retrieval Provider
type ProviderRequestValidator struct {
	paymentAddr   address.Address
	storageDeals  repo.StorageDealRepo
	pieceInfo     *PieceInfo
	retrievalDeal repo.IRetrievalDealRepo
	retrievalAsk  repo.IRetrievalAskRepo
}

// NewProviderRequestValidator returns a new instance of the ProviderRequestValidator
func NewProviderRequestValidator(paymentAddr address.Address, storageDeals repo.StorageDealRepo, retrievalDeal repo.IRetrievalDealRepo, retrievalAsk repo.IRetrievalAskRepo, pieceInfo *PieceInfo) *ProviderRequestValidator {
	return &ProviderRequestValidator{paymentAddr: paymentAddr, storageDeals: storageDeals, retrievalDeal: retrievalDeal, retrievalAsk: retrievalAsk, pieceInfo: pieceInfo}
}

// ValidatePush validates a push request received from the peer that will send data
func (rv *ProviderRequestValidator) ValidatePush(isRestart bool, _ datatransfer.ChannelID, sender peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) (datatransfer.VoucherResult, error) {
	return nil, errors.New("no pushes accepted")
}

// ValidatePull validates a pull request received from the peer that will receive data
func (rv *ProviderRequestValidator) ValidatePull(isRestart bool, _ datatransfer.ChannelID, receiver peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) (datatransfer.VoucherResult, error) {
	ctx := context.TODO()
	proposal, ok := voucher.(*retrievalmarket.DealProposal)
	var legacyProtocol bool
	if !ok {
		legacyProposal, ok := voucher.(*migrations.DealProposal0)
		if !ok {
			return nil, errors.New("wrong voucher type")
		}
		newProposal := migrations.MigrateDealProposal0To1(*legacyProposal)
		proposal = &newProposal
		legacyProtocol = true
	}
	response, err := rv.validatePull(ctx, isRestart, receiver, proposal, legacyProtocol, baseCid, selector)
	if response == nil {
		return nil, err
	}
	if legacyProtocol {
		downgradedResponse := migrations.DealResponse0{
			Status:      response.Status,
			ID:          response.ID,
			Message:     response.Message,
			PaymentOwed: response.PaymentOwed,
		}
		return &downgradedResponse, err
	}
	return response, err
}

// validatePull is called by the data provider when a new graphsync pull
// request is created. This can be the initial pull request or a new request
// created when the data transfer is restarted (eg after a connection failure).
// By default the graphsync request starts immediately sending data, unless
// validatePull returns ErrPause or the data-transfer has not yet started
// (because the provider is still unsealing the data).
func (rv *ProviderRequestValidator) validatePull(ctx context.Context, isRestart bool, receiver peer.ID, proposal *retrievalmarket.DealProposal, legacyProtocol bool, baseCid cid.Cid, selector ipld.Node) (*retrievalmarket.DealResponse, error) {
	// Check the proposal CID matches
	if proposal.PayloadCID != baseCid {
		return nil, errors.New("incorrect CID for this proposal")
	}

	// Check the proposal selector matches
	buf := new(bytes.Buffer)
	err := dagcbor.Encode(selector, buf)
	if err != nil {
		return nil, err
	}
	bytesCompare := allSelectorBytes
	if proposal.SelectorSpecified() {
		bytesCompare = proposal.Selector.Raw
	}
	if !bytes.Equal(buf.Bytes(), bytesCompare) {
		return nil, errors.New("incorrect selector for this proposal")
	}

	// If the validation is for a restart request, return nil, which means
	// the data-transfer should not be explicitly paused or resumed
	if isRestart {
		return nil, nil
	}

	// This is a new graphsync request (not a restart)
	pds := types.ProviderDealState{
		DealProposal:    *proposal,
		Receiver:        receiver,
		LegacyProtocol:  legacyProtocol,
		CurrentInterval: proposal.PaymentInterval,
	}

	// Decide whether to accept the deal
	status, err := rv.acceptDeal(ctx, &pds)

	response := retrievalmarket.DealResponse{
		ID:     proposal.ID,
		Status: status,
	}

	if status == retrievalmarket.DealStatusFundsNeededUnseal {
		response.PaymentOwed = pds.UnsealPrice
	}

	if err != nil {
		response.Message = err.Error()
		return &response, err
	}

	if pds.UnsealPrice.GreaterThan(big.Zero()) {
		pds.Status = retrievalmarket.DealStatusFundsNeededUnseal
		pds.TotalSent = 0

	} else {
		pds.TotalSent = 0
		pds.FundsReceived = abi.NewTokenAmount(0)
	}

	err = rv.retrievalDeal.SaveDeal(ctx, &pds)
	if err != nil {
		response.Message = err.Error()
		return &response, err
	}

	// Pause the data transfer while unsealing the data.
	// The state machine will unpause the transfer when unsealing completes.
	return &response, datatransfer.ErrPause
}

func (rv *ProviderRequestValidator) acceptDeal(ctx context.Context, deal *types.ProviderDealState) (retrievalmarket.DealStatus, error) {
	minerdeals, err := rv.pieceInfo.GetPieceInfoFromCid(ctx, deal.PayloadCID, deal.PieceCID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return retrievalmarket.DealStatusDealNotFound, err
		}
		return retrievalmarket.DealStatusErrored, err
	}

	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()

	//todo how to select deal
	deal.SelStorageProposalCid = minerdeals[0].ProposalCid
	ask, err := rv.retrievalAsk.GetAsk(ctx, rv.paymentAddr)
	if err != nil {
		return retrievalmarket.DealStatusErrored, err
	}

	// check that the deal parameters match our required parameters or
	// reject outright
	err = CheckDealParams(ask, deal.PricePerByte, deal.PaymentInterval, deal.PaymentIntervalIncrease, deal.UnsealPrice)
	if err != nil {
		return retrievalmarket.DealStatusRejected, err
	}

	if deal.UnsealPrice.GreaterThan(big.Zero()) {
		return retrievalmarket.DealStatusFundsNeededUnseal, nil
	}

	return retrievalmarket.DealStatusAccepted, nil
}
