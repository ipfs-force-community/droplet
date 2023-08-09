package retrievalprovider

import (
	"context"
	"errors"
	"time"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/hannahhoward/go-pubsub"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/repo"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

var allSelector = selectorparse.CommonSelector_ExploreAllRecursively

var askTimeout = 5 * time.Second

// ProviderRequestValidator validates incoming requests for the Retrieval Provider
type ProviderRequestValidator struct {
	cfg           *config.MarketConfig
	storageDeals  repo.StorageDealRepo
	pieceInfo     *PieceInfo
	retrievalDeal repo.IRetrievalDealRepo
	retrievalAsk  repo.IRetrievalAskRepo
	rdf           config.RetrievalDealFilter
	psub          *pubsub.PubSub
}

// NewProviderRequestValidator returns a new instance of the ProviderRequestValidator
func NewProviderRequestValidator(
	cfg *config.MarketConfig,
	storageDeals repo.StorageDealRepo,
	retrievalDeal repo.IRetrievalDealRepo,
	retrievalAsk repo.IRetrievalAskRepo,
	pieceInfo *PieceInfo,
	rdf config.RetrievalDealFilter,
) *ProviderRequestValidator {
	return &ProviderRequestValidator{
		cfg:           cfg,
		storageDeals:  storageDeals,
		retrievalDeal: retrievalDeal,
		retrievalAsk:  retrievalAsk,
		pieceInfo:     pieceInfo,
		rdf:           rdf,
		psub:          pubsub.New(queryValidationDispatcher),
	}
}

// ValidatePush validates a push request received from the peer that will send data
func (rv *ProviderRequestValidator) ValidatePush(_ datatransfer.ChannelID, sender peer.ID, voucher datamodel.Node, baseCid cid.Cid, selector datamodel.Node) (datatransfer.ValidationResult, error) {
	return datatransfer.ValidationResult{}, errors.New("no pushes accepted")
}

// ValidatePull validates a pull request received from the peer that will receive data
func (rv *ProviderRequestValidator) ValidatePull(_ datatransfer.ChannelID, receiver peer.ID, voucher datamodel.Node, baseCid cid.Cid, selector datamodel.Node) (datatransfer.ValidationResult, error) {
	ctx := context.TODO()
	proposal, err := retrievalmarket.DealProposalFromNode(voucher)
	if err != nil {
		return datatransfer.ValidationResult{}, err
	}

	response, err := rv.validatePull(ctx, receiver, proposal, baseCid, selector)
	rv.publishValidationEvent(false, receiver, proposal, baseCid, selector, response, err)

	return response, err
}

func (rv *ProviderRequestValidator) publishValidationEvent(restart bool, receiver peer.ID, proposal *retrievalmarket.DealProposal, baseCid cid.Cid, selector datamodel.Node, response datatransfer.ValidationResult, err error) {
	var dealResponse *retrievalmarket.DealResponse
	if response.VoucherResult != nil {
		dealResponse, _ = retrievalmarket.DealResponseFromNode(response.VoucherResult.Voucher)
	}
	if err == nil && !response.Accepted {
		err = datatransfer.ErrRejected
	}
	_ = rv.psub.Publish(retrievalmarket.ProviderValidationEvent{
		IsRestart: false,
		Receiver:  receiver,
		Proposal:  proposal,
		BaseCid:   baseCid,
		Selector:  selector,
		Response:  dealResponse,
		Error:     err,
	})
}

func errorProposal(proposal *retrievalmarket.DealProposal, status retrievalmarket.DealStatus, reason string) (datatransfer.ValidationResult, error) {
	dr := retrievalmarket.DealResponse{
		ID:      proposal.ID,
		Status:  status,
		Message: reason,
	}
	node := retrievalmarket.BindnodeRegistry.TypeToNode(&dr)
	return datatransfer.ValidationResult{
		Accepted:      false,
		VoucherResult: &datatransfer.TypedVoucher{Voucher: node, Type: retrievalmarket.DealResponseType},
	}, nil
}

// validatePull is called by the data provider when a new graphsync pull
// request is created. This can be the initial pull request or a new request
// created when the data transfer is restarted (eg after a connection failure).
// By default the graphsync request starts immediately sending data, unless
// validatePull returns ErrPause or the data-transfer has not yet started
// (because the provider is still unsealing the data).
func (rv *ProviderRequestValidator) validatePull(ctx context.Context, receiver peer.ID, proposal *retrievalmarket.DealProposal, baseCid cid.Cid, selector ipld.Node) (datatransfer.ValidationResult, error) {
	// Check the proposal CID matches
	if proposal.PayloadCID != baseCid {
		return errorProposal(proposal, retrievalmarket.DealStatusRejected, "incorrect CID for this proposal")
	}

	// Check the proposal selector matches
	sel := allSelector
	if proposal.SelectorSpecified() {
		sel = proposal.Selector.Node
	}
	if !ipld.DeepEqual(sel, selector) {
		return errorProposal(proposal, retrievalmarket.DealStatusRejected, "incorrect selector specified for this proposal")
	}

	// This is a new graphsync request (not a restart)
	pds := types.ProviderDealState{
		DealProposal: *proposal,
		Receiver:     receiver,
	}

	// Decide whether to accept the deal
	status, err := rv.acceptDeal(ctx, &pds)
	if err != nil {
		return errorProposal(proposal, status, err.Error())
	}
	dr := retrievalmarket.DealResponse{
		ID:          proposal.ID,
		Status:      status,
		PaymentOwed: pds.Params.OutstandingBalance(big.Zero(), 0, false),
	}

	err = rv.retrievalDeal.SaveDeal(ctx, &pds)
	if err != nil {
		return errorProposal(proposal, status, err.Error())
	}

	// Pause the data transfer while unsealing the data.
	// The state machine will unpause the transfer when unsealing completes.
	node := retrievalmarket.BindnodeRegistry.TypeToNode(&dr)
	result := datatransfer.ValidationResult{
		Accepted:             true,
		VoucherResult:        &datatransfer.TypedVoucher{Voucher: node, Type: retrievalmarket.DealResponseType},
		ForcePause:           true,
		DataLimit:            pds.Params.NextInterval(big.Zero()),
		RequiresFinalization: true,
	}
	return result, nil
}

func (rv *ProviderRequestValidator) runDealDecisionLogic(ctx context.Context, deal *types.ProviderDealState) (bool, string, error) {
	if rv.rdf == nil {
		return true, "", nil
	}
	return rv.rdf(ctx, address.Undef, *deal)
}

func (rv *ProviderRequestValidator) acceptDeal(ctx context.Context, deal *types.ProviderDealState) (retrievalmarket.DealStatus, error) {
	minerDeals, err := rv.pieceInfo.GetPieceInfoFromCid(ctx, deal.PayloadCID, deal.PieceCID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return retrievalmarket.DealStatusDealNotFound, err
		}
		return retrievalmarket.DealStatusErrored, err
	}

	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()

	//todo this deal may not match with query ask, no way to get miner id in current protocol
	var ask *types.RetrievalAsk
	for _, minerDeal := range minerDeals {
		minerCfg, err := rv.cfg.MinerProviderConfig(minerDeal.Proposal.Provider, true)
		if err != nil {
			continue
		}
		if minerCfg.RetrievalPaymentAddress.Unwrap().Empty() {
			continue
		}
		deal.SelStorageProposalCid = minerDeal.ProposalCid
		ask, err = rv.retrievalAsk.GetAsk(ctx, minerDeal.Proposal.Provider)
		if err != nil {
			log.Warnf("got %s ask failed: %v", minerDeal.Proposal.Provider, err)
		} else {
			break
		}
	}
	if ask == nil {
		return retrievalmarket.DealStatusErrored, err
	}

	// check that the deal parameters match our required parameters or
	// reject outright
	err = CheckDealParams(ask, deal.PricePerByte, deal.PaymentInterval, deal.PaymentIntervalIncrease, deal.UnsealPrice)
	if err != nil {
		return retrievalmarket.DealStatusRejected, err
	}

	// todo: 检索订单的 `miner` 从哪里来?
	accepted, reason, err := rv.runDealDecisionLogic(ctx, deal)
	if err != nil {
		return retrievalmarket.DealStatusErrored, err
	}
	if !accepted {
		return retrievalmarket.DealStatusRejected, errors.New(reason)
	}

	if deal.UnsealPrice.GreaterThan(big.Zero()) {
		return retrievalmarket.DealStatusFundsNeededUnseal, nil
	}

	return retrievalmarket.DealStatusAccepted, nil
}

// ValidateRestart validates a request on restart, based on its current state
func (rv *ProviderRequestValidator) ValidateRestart(_ datatransfer.ChannelID, channelState datatransfer.ChannelState) (datatransfer.ValidationResult, error) {
	voucher := channelState.Voucher()
	proposal, err := retrievalmarket.DealProposalFromNode(voucher.Voucher)
	if err != nil {
		return datatransfer.ValidationResult{}, errors.New("wrong voucher type")
	}
	ctx := context.TODO()
	response, err := rv.validateRestart(ctx, proposal, channelState)
	rv.publishValidationEvent(true, channelState.OtherPeer(), proposal, channelState.BaseCID(), channelState.Selector(), response, err)
	return response, err
}

func (rv *ProviderRequestValidator) validateRestart(ctx context.Context, proposal *retrievalmarket.DealProposal, channelState datatransfer.ChannelState) (datatransfer.ValidationResult, error) {
	dealID := retrievalmarket.ProviderDealIdentifier{DealID: proposal.ID, Receiver: channelState.OtherPeer()}

	// read the deal state
	deal, err := rv.retrievalDeal.GetDeal(ctx, channelState.OtherPeer(), proposal.ID)
	if err != nil {
		return errorDealResponse(dealID, err), nil
	}

	// produce validation based on current deal state and channel state
	return datatransfer.ValidationResult{
		Accepted:             true,
		ForcePause:           deal.Status == retrievalmarket.DealStatusUnsealing || deal.Status == retrievalmarket.DealStatusFundsNeededUnseal,
		RequiresFinalization: requiresFinalization(deal, channelState),
		DataLimit:            deal.Params.NextInterval(deal.FundsReceived),
	}, nil
}

// requiresFinalization is true unless the deal is in finalization and no further funds are owed
func requiresFinalization(deal *types.ProviderDealState, channelState datatransfer.ChannelState) bool {
	if deal.Status != retrievalmarket.DealStatusFundsNeededLastPayment && deal.Status != retrievalmarket.DealStatusFinalizing {
		return true
	}
	owed := deal.Params.OutstandingBalance(deal.FundsReceived, channelState.Queued(), channelState.Status().InFinalization())
	return owed.GreaterThan(big.Zero())
}

func errorDealResponse(dealID retrievalmarket.ProviderDealIdentifier, err error) datatransfer.ValidationResult {
	dr := retrievalmarket.DealResponse{
		ID:      dealID.DealID,
		Message: err.Error(),
		Status:  retrievalmarket.DealStatusErrored,
	}
	log.Errorf("error proposal, id: %v, status: %v, reason: %v", dr.ID, dr.Status, dr.Message)
	node := retrievalmarket.BindnodeRegistry.TypeToNode(&dr)
	return datatransfer.ValidationResult{
		Accepted:      false,
		VoucherResult: &datatransfer.TypedVoucher{Voucher: node, Type: retrievalmarket.DealResponseType},
	}
}

func (rv *ProviderRequestValidator) Subscribe(subscriber retrievalmarket.ProviderValidationSubscriber) retrievalmarket.Unsubscribe {
	return retrievalmarket.Unsubscribe(rv.psub.Subscribe(subscriber))
}

func queryValidationDispatcher(evt pubsub.Event, subscriberFn pubsub.SubscriberFn) error {
	e, ok := evt.(retrievalmarket.ProviderValidationEvent)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb, ok := subscriberFn.(retrievalmarket.ProviderValidationSubscriber)
	if !ok {
		return errors.New("wrong type of callback")
	}
	cb(e)
	return nil
}
