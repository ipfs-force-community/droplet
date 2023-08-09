package retrievalprovider

import (
	"context"
	"errors"
	"fmt"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	vtypes "github.com/filecoin-project/venus/venus-shared/types"
	gtypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
	mktypes "github.com/filecoin-project/venus/venus-shared/types/market"

	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statemachine"
	"github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
)

type IRetrievalHandler interface {
	UnsealData(ctx context.Context, deal *mktypes.ProviderDealState) error
	CancelDeal(ctx context.Context, deal *mktypes.ProviderDealState) error
	CleanupDeal(ctx context.Context, deal *mktypes.ProviderDealState) error
	UpdateFunding(ctx context.Context, deal *mktypes.ProviderDealState) error
	Error(ctx context.Context, deal *mktypes.ProviderDealState, err error) error
}

var _ IRetrievalHandler = (*RetrievalDealHandler)(nil)

type RetrievalDealHandler struct {
	env                 ProviderDealEnvironment
	retrievalDealStore  repo.IRetrievalDealRepo
	storageDealRepo     repo.StorageDealRepo
	gatewayMarketClient gateway.IMarketClient
	pieceStorageMgr     *piecestorage.PieceStorageManager
}

func NewRetrievalDealHandler(env ProviderDealEnvironment, retrievalDealStore repo.IRetrievalDealRepo, storageDealRepo repo.StorageDealRepo, gatewayMarketClient gateway.IMarketClient, pieceStorageMgr *piecestorage.PieceStorageManager) IRetrievalHandler {
	return &RetrievalDealHandler{
		env:                 env,
		retrievalDealStore:  retrievalDealStore,
		storageDealRepo:     storageDealRepo,
		gatewayMarketClient: gatewayMarketClient,
		pieceStorageMgr:     pieceStorageMgr,
	}
}

func (p *RetrievalDealHandler) UnsealData(ctx context.Context, providerDeal *mktypes.ProviderDealState) (err error) {
	log := log.With("dealId", providerDeal.ID)
	providerDeal.Status = rm.DealStatusUnsealing
	err = p.retrievalDealStore.SaveDeal(ctx, providerDeal)
	if err != nil {
		return
	}

	deal, err := p.storageDealRepo.GetDeal(ctx, providerDeal.SelStorageProposalCid)
	if err != nil {
		return
	}

	pieceCid := deal.Proposal.PieceCID
	log = log.With("pieceCid", pieceCid)

	// check piece exist

	st, err := p.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		// check fail, but unseal should continue
		log.Infof("try to find piece  fail: %w", err)
	}

	if st != nil {
		log.Info("piece already exist, no need to unseal")
	} else {
		// try unseal
		var wps piecestorage.IPieceStorage
		wps, err = p.pieceStorageMgr.FindStorageForWrite(int64(deal.Proposal.PieceSize))
		if err != nil {
			err = fmt.Errorf("failed to find storage to write %s: %w", deal.Proposal.PieceCID, err)
			return
		}

		var pieceTransfer string
		pieceTransfer, err = wps.GetPieceTransfer(ctx, pieceCid.String())
		if err != nil {
			err = fmt.Errorf("get piece transfer for %s: %w", pieceCid, err)
			return
		}

		log.Info("try to unseal")
		// should block util unseal finish or error, because it will resume transfer later
		state := gtypes.UnsealStateFailed
		checkUnsealInterval := 5 * time.Minute
		ticker := time.NewTicker(checkUnsealInterval)
		defer ticker.Stop()
		timeOutCtx, cancel := context.WithTimeout(ctx, 12*time.Hour)
		defer cancel()

		errRetry, errRetryCount := 5, 0

	CheckLoop:
		for state != gtypes.UnsealStateFinished {
			state, err = p.gatewayMarketClient.SectorsUnsealPiece(
				ctx,
				deal.Proposal.Provider,
				pieceCid,
				deal.SectorNumber,
				vtypes.UnpaddedByteIndex(deal.Offset.Unpadded()),
				deal.Proposal.PieceSize.Unpadded(),
				pieceTransfer,
			)
			if err != nil {
				err = fmt.Errorf("unseal piece %s: %w", pieceCid, err)
				errRetryCount++
				log.Warnf("unseal piece %s fail, retry (%d/%d): %w", pieceCid, errRetryCount, errRetry, err)
				if errRetryCount > errRetry {
					return
				}
			}
			log.Debugf("unseal piece %s: %s", pieceCid, state)
			switch state {
			case gtypes.UnsealStateFailed:
				err = fmt.Errorf("unseal piece %s fail: %w", pieceCid, err)
				return
			case gtypes.UnsealStateFinished:
				break CheckLoop
			}
			select {
			case <-ticker.C:
			case <-timeOutCtx.Done():
				err = ctx.Err()
				return
			}
		}
		log.Info("unseal piece success")
	}

	if err = p.env.PrepareBlockstore(ctx, providerDeal.ID, deal.Proposal.PieceCID); err != nil {
		log.Errorf("unable to load shard %s  %s", deal.Proposal.PieceCID, err.Error())
		err = p.CancelDeal(ctx, providerDeal)
		return
	}
	log.Debugf("blockstore prepared successfully, firing unseal complete for deal %d", providerDeal.ID)
	providerDeal.Status = rm.DealStatusUnsealed
	err = p.retrievalDealStore.SaveDeal(ctx, providerDeal)
	if err != nil {
		return
	}

	log.Debugf("unpausing data transfer for deal %d", providerDeal.ID)

	if providerDeal.ChannelID != nil {
		log.Debugf("resuming data transfer for deal %d", providerDeal.ID)
		err = p.env.ResumeDataTransfer(ctx, *providerDeal.ChannelID)
		if err != nil {
			providerDeal.Status = rm.DealStatusErrored
		}
	}
	err = p.retrievalDealStore.SaveDeal(ctx, providerDeal)
	return
}

func (p *RetrievalDealHandler) CancelDeal(ctx context.Context, deal *mktypes.ProviderDealState) error {
	// Read next response (or fail)
	err := p.env.DeleteStore(deal.ID)
	if err != nil {
		return p.Error(ctx, deal, nil)
	}
	if deal.ChannelID != nil {
		err = p.env.CloseDataTransfer(ctx, *deal.ChannelID)
		if err != nil && !errors.Is(err, statemachine.ErrTerminated) {
			return p.Error(ctx, deal, nil)
		}
	}
	deal.Status = rm.DealStatusCancelled
	return p.retrievalDealStore.SaveDeal(ctx, deal)
}

// CleanupDeal runs to do memory cleanup for an in progress deal
func (p *RetrievalDealHandler) CleanupDeal(ctx context.Context, deal *mktypes.ProviderDealState) error {
	err := p.env.DeleteStore(deal.ID)
	if err != nil {
		return p.Error(ctx, deal, nil)
	}
	deal.Status = rm.DealStatusCompleted
	return p.retrievalDealStore.SaveDeal(ctx, deal)
}

func (p *RetrievalDealHandler) Error(ctx context.Context, deal *mktypes.ProviderDealState, err error) error {
	deal.Status = rm.DealStatusErrored
	if err != nil {
		deal.Message = err.Error()
	}
	return p.retrievalDealStore.SaveDeal(ctx, deal)
}

// UpdateFunding saves payments as needed until a transfer can resume
func (p *RetrievalDealHandler) UpdateFunding(ctx context.Context, deal *mktypes.ProviderDealState) error {
	log.Debugf("handling new event while in ongoing state of transfer %d", deal.ID)
	// if we have no channel ID yet, there's no need to attempt to process payment based on channel state
	if deal.ChannelID == nil {
		return nil
	}
	// read the channel state based on the channel id
	channelState, err := p.env.ChannelState(ctx, *deal.ChannelID)
	if err != nil {
		return p.Error(ctx, deal, err)
	}
	// process funding and produce the new validation status
	result := p.updateFunding(ctx, deal, channelState)
	// update the validation status on the channel
	err = p.env.UpdateValidationStatus(ctx, *deal.ChannelID, result)
	if err != nil {
		return p.Error(ctx, deal, err)
	}
	return nil
}

func (p *RetrievalDealHandler) updateFunding(ctx context.Context,
	deal *mktypes.ProviderDealState,
	channelState datatransfer.ChannelState,
) datatransfer.ValidationResult {
	// process payment, determining how many more funds we have then the current deal.FundsReceived
	received, err := p.processLastVoucher(ctx, channelState, deal)
	if err != nil {
		return errorDealResponse(deal.Identifier(), err)
	}

	if received.Nil() {
		received = big.Zero()
	}

	// calculate the current amount paid
	totalPaid := big.Add(deal.FundsReceived, received)

	// check whether money is owed based on deal parameters, total amount paid, and current state of the transfer
	owed := deal.Params.OutstandingBalance(totalPaid, channelState.Queued(), channelState.Status().InFinalization())
	log.Debugf("provider: owed %d, total received %d = received so far %d + newly received %d, unseal price %d, price per byte %d, bytes sent: %d, in finalization: %v",
		owed, totalPaid, deal.FundsReceived, received, deal.UnsealPrice, deal.PricePerByte, channelState.Queued(), channelState.Status().InFinalization())

	var voucherResult *rm.DealResponse
	if owed.GreaterThan(big.Zero()) {
		// if payment is still owed but we received funds, send a partial payment received event
		if received.GreaterThan(big.Zero()) {
			log.Debugf("provider: owed %d: sending partial payment request", owed)
			deal.FundsReceived = big.Add(deal.FundsReceived, received)
			if err := p.retrievalDealStore.SaveDeal(ctx, deal); err != nil {
				log.Errorf("save deal FundsReceived failed: %v", err)
			}
		}
		// sending this response voucher is primarily to cover for current client logic --
		// our client expects a voucher requesting payment before it sends anything
		// TODO: remove this when the client no longer expects a voucher
		if received.GreaterThan(big.Zero()) || deal.Status != rm.DealStatusFundsNeededUnseal {
			voucherResult = &rm.DealResponse{
				ID:          deal.ID,
				Status:      deal.Status,
				PaymentOwed: owed,
			}
		}
	} else {
		// send an event to record payment received
		deal.FundsReceived = big.Add(deal.FundsReceived, received)
		if err := p.retrievalDealStore.SaveDeal(ctx, deal); err != nil {
			log.Errorf("save deal FundsReceived failed: %v", err)
		}
		if deal.Status == rm.DealStatusFundsNeededLastPayment {
			log.Debugf("provider: funds needed: last payment")
			// sending this response voucher is primarily to cover for current client logic --
			// our client expects a voucher announcing completion from the provider before it finishes
			// TODO: remove this when the current no longer expects a voucher
			voucherResult = &rm.DealResponse{
				ID:     deal.ID,
				Status: rm.DealStatusCompleted,
			}
		}
	}

	vr := datatransfer.ValidationResult{
		Accepted:             true,
		ForcePause:           deal.Status == rm.DealStatusUnsealing || deal.Status == rm.DealStatusFundsNeededUnseal,
		RequiresFinalization: owed.GreaterThan(big.Zero()) || deal.Status != rm.DealStatusFundsNeededLastPayment,
		DataLimit:            deal.Params.NextInterval(totalPaid),
	}
	if voucherResult != nil {
		node := rm.BindnodeRegistry.TypeToNode(voucherResult)
		vr.VoucherResult = &datatransfer.TypedVoucher{Voucher: node, Type: rm.DealResponseType}
	}
	return vr
}

func (p *RetrievalDealHandler) savePayment(ctx context.Context, payment *rm.DealPayment, deal *mktypes.ProviderDealState) (abi.TokenAmount, error) {
	tok, _, err := p.env.GetChainHead(context.TODO())
	if err != nil {
		_ = p.CancelDeal(ctx, deal)
		return big.Zero(), err
	}
	// Save voucher
	received, err := p.env.SavePaymentVoucher(context.TODO(), payment.PaymentChannel, payment.PaymentVoucher, nil, big.Zero(), tok)
	if err != nil {
		_ = p.CancelDeal(ctx, deal)
		return big.Zero(), fmt.Errorf("save payment voucher failed: %v", err)
	}
	return received, nil
}

func (p *RetrievalDealHandler) processLastVoucher(ctx context.Context, channelState datatransfer.ChannelState, deal *mktypes.ProviderDealState) (abi.TokenAmount, error) {
	voucher := channelState.LastVoucher()

	// read payment and return response if present
	if payment, err := rm.DealPaymentFromNode(voucher.Voucher); err == nil {
		return p.savePayment(ctx, payment, deal)
	}

	if _, err := rm.DealProposalFromNode(voucher.Voucher); err == nil {
		return big.Zero(), nil
	}

	return big.Zero(), errors.New("wrong voucher type")
}
