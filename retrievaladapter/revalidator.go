package retrievaladapter

import (
	"context"
	"errors"
	"sync"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/migrations"
)

type channelData struct {
	dealID         rm.ProviderDealIdentifier
	totalSent      uint64
	totalPaidFor   uint64
	interval       uint64
	pricePerByte   abi.TokenAmount
	reload         bool
	legacyProtocol bool
}

// ProviderRevalidator defines data transfer revalidation logic in the context of
// a provider for a retrieval deal
type ProviderRevalidator struct {
	node                 rm.RetrievalProviderNode
	deals                RetrievalDealStore
	retrievalDealHandler IRetrievalHandler
	trackedChannelsLk    sync.RWMutex
	trackedChannels      map[datatransfer.ChannelID]*channelData
}

// NewProviderRevalidator returns a new instance of a ProviderRevalidator
func NewProviderRevalidator(node rm.RetrievalProviderNode, deals RetrievalDealStore, retrievalDealHandler IRetrievalHandler) *ProviderRevalidator {
	return &ProviderRevalidator{
		node:                 node,
		deals:                deals,
		retrievalDealHandler: retrievalDealHandler,
		trackedChannels:      make(map[datatransfer.ChannelID]*channelData),
	}
}

// TrackChannel indicates a retrieval deal tracked by this provider. It associates
// a given channel ID with a retrieval deal, so that checks run for data sent
// on the channel
func (pr *ProviderRevalidator) TrackChannel(deal rm.ProviderDealState) {
	if deal.ChannelID == nil {
		return
	}

	pr.trackedChannelsLk.Lock()
	defer pr.trackedChannelsLk.Unlock()
	pr.trackedChannels[*deal.ChannelID] = &channelData{
		dealID: deal.Identifier(),
	}
	pr.writeDealState(&deal)
}

// UntrackChannel indicates a retrieval deal is finish and no longer is tracked
// by this provider
func (pr *ProviderRevalidator) UntrackChannel(deal rm.ProviderDealState) {
	// Sanity check
	if deal.ChannelID == nil {
		log.Errorf("cannot untrack deal %s: channel ID is nil", deal.ID)
		return
	}

	pr.trackedChannelsLk.Lock()
	defer pr.trackedChannelsLk.Unlock()
	delete(pr.trackedChannels, *deal.ChannelID)
}

func (pr *ProviderRevalidator) loadDealState(channel *channelData) error {
	if !channel.reload {
		return nil
	}
	deal, err := pr.deals.GetDeal(channel.dealID.Receiver, channel.dealID.DealID)
	if err != nil {
		return err
	}
	pr.writeDealState(deal)
	channel.reload = false
	return nil
}

func (pr *ProviderRevalidator) writeDealState(deal *rm.ProviderDealState) {
	channel := pr.trackedChannels[*deal.ChannelID]
	channel.totalSent = deal.TotalSent
	if !deal.PricePerByte.IsZero() {
		channel.totalPaidFor = big.Div(big.Max(big.Sub(deal.FundsReceived, deal.UnsealPrice), big.Zero()), deal.PricePerByte).Uint64()
	}
	channel.interval = deal.CurrentInterval
	channel.pricePerByte = deal.PricePerByte
	channel.legacyProtocol = deal.LegacyProtocol
}

// Revalidate revalidates a request with a new voucher
func (pr *ProviderRevalidator) Revalidate(channelID datatransfer.ChannelID, voucher datatransfer.Voucher) (datatransfer.VoucherResult, error) {
	pr.trackedChannelsLk.RLock()
	defer pr.trackedChannelsLk.RUnlock()
	ctx := context.TODO()
	channel, ok := pr.trackedChannels[channelID]
	if !ok {
		return nil, nil
	}

	// read payment, or fail
	payment, ok := voucher.(*rm.DealPayment)
	var legacyProtocol bool
	if !ok {
		legacyPayment, ok := voucher.(*migrations.DealPayment0)
		if !ok {
			return nil, errors.New("wrong voucher type")
		}
		newPayment := migrations.MigrateDealPayment0To1(*legacyPayment)
		payment = &newPayment
		legacyProtocol = true
	}

	response, err := pr.processPayment(ctx, channel.dealID, payment)
	if err == nil || err == datatransfer.ErrResume {
		channel.reload = true
	}
	return finalResponse(response, legacyProtocol), err
}

func (pr *ProviderRevalidator) processPayment(ctx context.Context, dealID rm.ProviderDealIdentifier, payment *rm.DealPayment) (*retrievalmarket.DealResponse, error) {
	deal, err := pr.deals.GetDeal(dealID.Receiver, dealID.DealID)
	if err != nil {
		//todo if getdeal fail, cancel deal fail too. how to resolve this issue, need to think
		_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
		return errorDealResponse(dealID, err), err
	}

	tok, _, err := pr.node.GetChainHead(context.TODO())
	if err != nil {
		_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
		return errorDealResponse(dealID, err), err
	}

	// Save voucher
	received, err := pr.node.SavePaymentVoucher(context.TODO(), payment.PaymentChannel, payment.PaymentVoucher, nil, big.Zero(), tok)
	if err != nil {
		_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
		return errorDealResponse(dealID, err), err
	}

	totalPaid := big.Add(deal.FundsReceived, received)

	// check if all payments are received to continue the deal, or send updated required payment
	owed := paymentOwed(deal, totalPaid)

	log.Debugf("provider: owed %d: received voucher for %d, total received %d = received so far %d + newly received %d, total sent %d, unseal price %d, price per byte %d",
		owed, payment.PaymentVoucher.Amount, totalPaid, deal.FundsReceived, received, deal.TotalSent, deal.UnsealPrice, deal.PricePerByte)

	if owed.GreaterThan(big.Zero()) {
		log.Debugf("provider: owed %d: sending partial payment request", owed)
		deal.FundsReceived = big.Add(deal.FundsReceived, received)
		err := pr.deals.SaveDeal(deal)
		if err != nil {
			//todo  receive voucher save success, but track deal status failed
			//give error here may client send more funds than fact
			_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
			return errorDealResponse(dealID, err), err
		}
		return &rm.DealResponse{
			ID:          deal.ID,
			Status:      deal.Status,
			PaymentOwed: owed,
		}, datatransfer.ErrPause
	}

	// resume deal
	var sumPayment = func() {
		deal.FundsReceived = big.Add(deal.FundsReceived, received)

		// only update interval if the payment is for bytes and not for unsealing.
		if deal.Status != rm.DealStatusFundsNeededUnseal {
			deal.CurrentInterval = deal.NextInterval()
		}
	}
	var resp *retrievalmarket.DealResponse
	switch deal.Status {
	case rm.DealStatusFundsNeeded:
		sumPayment()
		deal.Status = rm.DealStatusOngoing
	case rm.DealStatusFundsNeededLastPayment:
		sumPayment()
		deal.Status = rm.DealStatusFinalizing
		log.Debugf("provider: funds needed: last payment")
		resp = &rm.DealResponse{
			ID:     deal.ID,
			Status: rm.DealStatusCompleted,
		}
		err = datatransfer.ErrResume
	case rm.DealStatusFundsNeededUnseal:
		sumPayment()
		deal.Status = rm.DealStatusUnsealing
		defer func() {
			go pr.retrievalDealHandler.UnsealData(ctx, deal)
		}()
	case rm.DealStatusBlocksComplete, rm.DealStatusOngoing, rm.DealStatusFinalizing:
		err = nil
	default:
		err = datatransfer.ErrResume
	}

	err = pr.deals.SaveDeal(deal)
	if err != nil {
		// todo can recover from storage error?
		_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
		return errorDealResponse(dealID, err), err
	}
	return resp, err
}

func paymentOwed(deal *rm.ProviderDealState, totalPaid big.Int) big.Int {
	// Check if the payment covers unsealing
	if totalPaid.LessThan(deal.UnsealPrice) {
		log.Debugf("provider: total paid %d < unseal price %d", totalPaid, deal.UnsealPrice)
		return big.Sub(deal.UnsealPrice, totalPaid)
	}

	// Calculate how much payment has been made for transferred data
	transferPayment := big.Sub(totalPaid, deal.UnsealPrice)

	// The provider sends data and the client sends payment for the data.
	// The provider will send a limited amount of extra data before receiving
	// payment. Given the current limit, check if the client has paid enough
	// to unlock the next interval.
	currentLimitLower := deal.IntervalLowerBound()

	log.Debugf("provider: total sent %d bytes, but require payment for interval lower bound %d bytes",
		deal.TotalSent, currentLimitLower)

	// Calculate the minimum required payment
	totalPaymentRequired := big.Mul(big.NewInt(int64(currentLimitLower)), deal.PricePerByte)

	// Calculate payment owed
	owed := big.Sub(totalPaymentRequired, transferPayment)
	log.Debugf("provider: payment owed %d = payment required %d - transfer paid %d",
		owed, totalPaymentRequired, transferPayment)

	return owed
}

func errorDealResponse(dealID rm.ProviderDealIdentifier, err error) *rm.DealResponse {
	return &rm.DealResponse{
		ID:      dealID.DealID,
		Message: err.Error(),
		Status:  rm.DealStatusErrored,
	}
}

// OnPullDataSent is called on the responder side when more bytes are sent
// for a given pull request. It should return a VoucherResult + ErrPause to
// request revalidation or nil to continue uninterrupted,
// other errors will terminate the request
func (pr *ProviderRevalidator) OnPullDataSent(chid datatransfer.ChannelID, additionalBytesSent uint64) (bool, datatransfer.VoucherResult, error) {
	ctx := context.TODO()
	pr.trackedChannelsLk.RLock()
	defer pr.trackedChannelsLk.RUnlock()
	channel, ok := pr.trackedChannels[chid]
	if !ok {
		return false, nil, nil
	}
	deal, err := pr.deals.GetDeal(channel.dealID.Receiver, channel.dealID.DealID)
	if err != nil {
		return true, nil, err
	}

	err = pr.loadDealState(channel)
	if err != nil {
		return true, nil, err
	}

	// Calculate how much data has been sent in total
	channel.totalSent += additionalBytesSent
	if channel.pricePerByte.IsZero() || channel.totalSent < channel.interval {
		if !channel.pricePerByte.IsZero() {
			log.Debugf("provider: total sent %d < interval %d, sending block", channel.totalSent, channel.interval)
		}
		deal.Status = rm.DealStatusOngoing
		deal.TotalSent = channel.totalSent
		return true, nil, pr.deals.SaveDeal(deal)
	}

	// Calculate the payment owed
	paymentOwed := big.Mul(abi.NewTokenAmount(int64(channel.totalSent-channel.totalPaidFor)), channel.pricePerByte)
	log.Debugf("provider: owed %d = (total sent %d - paid for %d) * price per byte %d: sending payment request",
		paymentOwed, channel.totalSent, channel.totalPaidFor, channel.pricePerByte)

	// Request payment
	switch deal.Status {
	case rm.DealStatusOngoing, rm.DealStatusUnsealed:
		deal.Status = rm.DealStatusFundsNeeded
		deal.TotalSent = channel.totalSent
	case rm.DealStatusFundsNeeded:
		//doing nothing
	case rm.DealStatusBlocksComplete:
		deal.Status = rm.DealStatusFundsNeededLastPayment
		deal.TotalSent = channel.totalSent
	case rm.DealStatusNew:
		//todo will come here?
		log.Errorf("receive status new on data pull sent")
		deal.Status = rm.DealStatusFundsNeededUnseal
		deal.TotalSent = channel.totalSent // total sent may not need, not unseal, send nothing
		err = pr.retrievalDealHandler.TrackTransfer(ctx, deal)
		if err != nil {
			return true, nil, err
		}
	}

	err = pr.deals.SaveDeal(deal)
	if err != nil {
		return true, nil, err
	}

	return true, finalResponse(&rm.DealResponse{
		ID:          channel.dealID.DealID,
		Status:      rm.DealStatusFundsNeeded,
		PaymentOwed: paymentOwed,
	}, channel.legacyProtocol), datatransfer.ErrPause
}

// OnPushDataReceived is called on the responder side when more bytes are received
// for a given push request.  It should return a VoucherResult + ErrPause to
// request revalidation or nil to continue uninterrupted,
// other errors will terminate the request
func (pr *ProviderRevalidator) OnPushDataReceived(chid datatransfer.ChannelID, additionalBytesReceived uint64) (bool, datatransfer.VoucherResult, error) {
	return false, nil, nil
}

// OnComplete is called to make a final request for revalidation -- often for the
// purpose of settlement.
// if VoucherResult is non nil, the request will enter a settlement phase awaiting
// a final update
func (pr *ProviderRevalidator) OnComplete(chid datatransfer.ChannelID) (bool, datatransfer.VoucherResult, error) {
	pr.trackedChannelsLk.RLock()
	defer pr.trackedChannelsLk.RUnlock()
	channel, ok := pr.trackedChannels[chid]
	if !ok {
		return false, nil, nil
	}
	deal, err := pr.deals.GetDeal(channel.dealID.Receiver, channel.dealID.DealID)
	if err != nil {
		return true, nil, err
	}

	err = pr.loadDealState(channel)
	if err != nil {
		return true, nil, err
	}

	deal.Status = rm.DealStatusBlocksComplete
	err = pr.deals.SaveDeal(deal)
	if err != nil {
		return true, nil, err
	}

	// Calculate how much payment is owed
	paymentOwed := big.Mul(abi.NewTokenAmount(int64(channel.totalSent-channel.totalPaidFor)), channel.pricePerByte)
	if paymentOwed.Equals(big.Zero()) {
		return true, finalResponse(&rm.DealResponse{
			ID:     channel.dealID.DealID,
			Status: rm.DealStatusCompleted,
		}, channel.legacyProtocol), nil
	}

	// Send a request for payment
	log.Debugf("provider: last payment owed %d = (total sent %d - paid for %d) * price per byte %d",
		paymentOwed, channel.totalSent, channel.totalPaidFor, channel.pricePerByte)
	deal.Status = rm.DealStatusFundsNeededLastPayment
	deal.TotalSent = channel.totalSent
	err = pr.deals.SaveDeal(deal)
	if err != nil {
		return true, nil, err
	}

	return true, finalResponse(&rm.DealResponse{
		ID:          channel.dealID.DealID,
		Status:      rm.DealStatusFundsNeededLastPayment,
		PaymentOwed: paymentOwed,
	}, channel.legacyProtocol), datatransfer.ErrPause
}

func finalResponse(response *rm.DealResponse, legacyProtocol bool) datatransfer.Voucher {
	if response == nil {
		return nil
	}
	if legacyProtocol {
		downgradedResponse := migrations.DealResponse0{
			Status:      response.Status,
			ID:          response.ID,
			Message:     response.Message,
			PaymentOwed: response.PaymentOwed,
		}
		return &downgradedResponse
	}
	return response
}

type legacyRevalidator struct {
	providerRevalidator *ProviderRevalidator
}

func (lrv *legacyRevalidator) Revalidate(channelID datatransfer.ChannelID, voucher datatransfer.Voucher) (datatransfer.VoucherResult, error) {
	return lrv.providerRevalidator.Revalidate(channelID, voucher)
}

func (lrv *legacyRevalidator) OnPullDataSent(chid datatransfer.ChannelID, additionalBytesSent uint64) (bool, datatransfer.VoucherResult, error) {
	return false, nil, nil
}

func (lrv *legacyRevalidator) OnPushDataReceived(chid datatransfer.ChannelID, additionalBytesReceived uint64) (bool, datatransfer.VoucherResult, error) {
	return false, nil, nil
}

func (lrv *legacyRevalidator) OnComplete(chid datatransfer.ChannelID) (bool, datatransfer.VoucherResult, error) {
	return false, nil, nil
}

// NewLegacyRevalidator adds a revalidator that will capture revalidation requests for the legacy protocol but
// won't double count data being sent
// TODO: the data transfer revalidator registration needs to be able to take multiple types to avoid double counting
// for data being sent.
func NewLegacyRevalidator(providerRevalidator *ProviderRevalidator) datatransfer.Revalidator {
	return &legacyRevalidator{providerRevalidator: providerRevalidator}
}
