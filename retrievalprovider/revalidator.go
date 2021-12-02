package retrievalprovider

import (
	"context"
	"errors"
	"fmt"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/paychmgr"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus/app/client/apiface"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/migrations"
)

// ProviderRevalidator defines data transfer revalidation logic in the context of
// a provider for a retrieval deal
type ProviderRevalidator struct {
	fullNode             apiface.FullNode
	payAPI               *paychmgr.PaychAPI
	deals                repo.IRetrievalDealRepo
	retrievalDealHandler IRetrievalHandler
}

// NewProviderRevalidator returns a new instance of a ProviderRevalidator
func NewProviderRevalidator(fullNode apiface.FullNode, payAPI *paychmgr.PaychAPI, deals repo.IRetrievalDealRepo, retrievalDealHandler IRetrievalHandler) *ProviderRevalidator {
	return &ProviderRevalidator{
		fullNode:             fullNode,
		payAPI:               payAPI,
		deals:                deals,
		retrievalDealHandler: retrievalDealHandler,
	}
}

// Revalidate revalidates a request with a new voucher
func (pr *ProviderRevalidator) Revalidate(channelID datatransfer.ChannelID, voucher datatransfer.Voucher) (datatransfer.VoucherResult, error) {
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
	ctx := context.TODO()
	log.Infof("receive payment %s", payment.ID)
	deal, err := pr.deals.GetDeal(channelID.Initiator, payment.ID)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, nil
		} else {
			_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
			return finalResponse(errorDealResponse(retrievalmarket.ProviderDealIdentifier{Receiver: channelID.Initiator, DealID: payment.ID}, err), legacyProtocol), err
		}
	}

	response, err := pr.processPayment(ctx, deal, payment)
	return finalResponse(response, legacyProtocol), err

}

func (pr *ProviderRevalidator) processPayment(ctx context.Context, deal *types.ProviderDealState, payment *rm.DealPayment) (*retrievalmarket.DealResponse, error) {
	// Save voucher
	received, err := pr.payAPI.PaychVoucherAdd(context.TODO(), payment.PaymentChannel, payment.PaymentVoucher, nil, big.Zero())
	if err != nil {
		_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
		return errorDealResponse(deal.Identifier(), err), err
	}

	totalPaid := big.Add(deal.FundsReceived, received)

	// check if all payments are received to continue the deal, or send updated required payment
	owed := paymentOwed(deal, totalPaid)

	log.Debugf("provider: owed %d: received voucher for %d, total received %d = received so far %d + newly received %d, total sent %d, unseal price %d, price per byte %d",
		owed, payment.PaymentVoucher.Amount, totalPaid, deal.FundsReceived, received, deal.TotalSent, deal.UnsealPrice, deal.PricePerByte)

	if owed.GreaterThan(big.Zero()) {
		log.Debugf("provider: owed %d: sending partial payment request", owed)
		deal.FundsReceived = big.Add(deal.FundsReceived, received)
		fmt.Println("receive fee ", big.Div(deal.FundsReceived, deal.PricePerByte))
		err := pr.deals.SaveDeal(deal)
		if err != nil {
			//todo  receive voucher save success, but track deal status failed
			//give error here may client send more funds than fact
			_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
			return errorDealResponse(deal.Identifier(), err), err
		}
		return &rm.DealResponse{
			ID:          deal.ID,
			Status:      deal.Status,
			PaymentOwed: owed,
		}, datatransfer.ErrPause
	}

	// resume deal
	deal.FundsReceived = big.Add(deal.FundsReceived, received)
	fmt.Println("receive fee ", big.Div(deal.FundsReceived, deal.PricePerByte))
	// only update interval if the payment is for bytes and not for unsealing.
	if deal.Status != rm.DealStatusFundsNeededUnseal {
		deal.CurrentInterval = deal.NextInterval()
	}

	fmt.Println("receive payment ", deal.Status.String())

	var resp *retrievalmarket.DealResponse
	err = datatransfer.ErrResume
	switch deal.Status {
	case rm.DealStatusFundsNeeded:
		deal.Status = rm.DealStatusOngoing
	case rm.DealStatusFundsNeededLastPayment:
		deal.Status = rm.DealStatusFinalizing
		log.Infof("provider: funds needed: last payment")
		resp = &rm.DealResponse{
			ID:     deal.ID,
			Status: rm.DealStatusCompleted,
		}
	//not start transfer data is unsealing
	case rm.DealStatusFundsNeededUnseal:
		//pay for unseal goto unseal
		deal.Status = rm.DealStatusUnsealing
		defer func() {
			go pr.retrievalDealHandler.UnsealData(ctx, deal) //nolint
		}()
		err = nil
	case rm.DealStatusUnsealing:
		err = nil
	}

	dErr := pr.deals.SaveDeal(deal)
	if dErr != nil {
		// todo can recover from storage error?
		_ = pr.retrievalDealHandler.CancelDeal(ctx, deal)
		return errorDealResponse(deal.Identifier(), dErr), err
	}
	return resp, err
}

func paymentOwed(deal *types.ProviderDealState, totalPaid big.Int) big.Int {
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
	fmt.Println("receive OnPullDataSent funx")
	deal, err := pr.deals.GetDealByTransferId(chid)
	if err != nil {
		if err == repo.ErrNotFound {
			return false, nil, nil
		}
		return true, nil, err
	}

	totalSent := deal.TotalSent
	totalPaidFor := deal.TotalPaidFor()

	// Calculate how much data has been sent in total
	totalSent += additionalBytesSent
	if deal.PricePerByte.IsZero() || totalSent < deal.CurrentInterval {
		if !deal.PricePerByte.IsZero() {
			log.Debugf("provider: total sent %d < interval %d, sending block", totalSent, deal.CurrentInterval)
		}
		deal.Status = rm.DealStatusOngoing
		deal.TotalSent = totalSent
		return true, nil, pr.deals.SaveDeal(deal)
	}

	// Calculate the payment owed
	paymentOwed := big.Mul(abi.NewTokenAmount(int64(totalSent-totalPaidFor)), deal.PricePerByte)
	log.Debugf("provider: owed %d = (total sent %d - paid for %d) * price per byte %d: sending payment request", paymentOwed, totalSent, totalPaidFor, deal.PricePerByte)

	deal.TotalSent = totalSent
	// Request payment
	switch deal.Status {
	case rm.DealStatusOngoing, rm.DealStatusUnsealed:
		deal.Status = rm.DealStatusFundsNeeded
	case rm.DealStatusFundsNeeded:
		//doing nothing
	case rm.DealStatusBlocksComplete:
		deal.Status = rm.DealStatusFundsNeededLastPayment
	case rm.DealStatusNew:
		//todo will come here?
		log.Errorf("receive status new on data pull sent")
		deal.Status = rm.DealStatusFundsNeededUnseal
	}

	err = pr.deals.SaveDeal(deal)
	if err != nil {
		return true, nil, err
	}

	return true, finalResponse(&rm.DealResponse{
		ID:          deal.DealProposal.ID,
		Status:      rm.DealStatusFundsNeeded,
		PaymentOwed: paymentOwed,
	}, deal.LegacyProtocol), datatransfer.ErrPause
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
	fmt.Println("receive oncomplete func")
	deal, err := pr.deals.GetDealByTransferId(chid)
	if err != nil {
		if err == repo.ErrNotFound {
			return false, nil, nil
		}
		return true, nil, err
	}

	deal.Status = rm.DealStatusBlocksComplete
	err = pr.deals.SaveDeal(deal)
	if err != nil {
		return true, nil, err
	}

	totalSent := deal.TotalSent
	totalPaidFor := deal.TotalPaidFor()
	// Calculate how much payment is owed
	paymentOwed := big.Mul(abi.NewTokenAmount(int64(totalSent-totalPaidFor)), deal.PricePerByte)
	if paymentOwed.Equals(big.Zero()) {
		log.Infof("OnComplete  xxxx")
		return true, finalResponse(&rm.DealResponse{
			ID:     deal.DealProposal.ID,
			Status: rm.DealStatusCompleted,
		}, deal.LegacyProtocol), nil
	}

	// Send a request for payment
	log.Debugf("provider: last payment owed %d = (total sent %d - paid for %d) * price per byte %d",
		paymentOwed, totalSent, totalPaidFor, deal.PricePerByte)
	deal.Status = rm.DealStatusFundsNeededLastPayment
	deal.TotalSent = totalSent
	err = pr.deals.SaveDeal(deal)
	if err != nil {
		return true, nil, err
	}

	return true, finalResponse(&rm.DealResponse{
		ID:          deal.DealProposal.ID,
		Status:      rm.DealStatusFundsNeededLastPayment,
		PaymentOwed: paymentOwed,
	}, deal.LegacyProtocol), datatransfer.ErrPause
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

// NewLegacyRevalidator adds a reValidator that will capture revalidation requests for the legacy protocol but
// won't double count data being sent
// TODO: the data transfer reValidator registration needs to be able to take multiple types to avoid double counting
// for data being sent.
func NewLegacyRevalidator(providerRevalidator *ProviderRevalidator) datatransfer.Revalidator {
	return &legacyRevalidator{providerRevalidator: providerRevalidator}
}
