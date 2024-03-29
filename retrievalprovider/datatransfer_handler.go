package retrievalprovider

import (
	"context"
	"fmt"

	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
)

// EventReceiver is any thing that can receive FSM events
type IDatatransferHandler interface {
	// have many receiver function
	HandleCompleteFor(context.Context, rm.ProviderDealIdentifier) error
	HandleAcceptFor(context.Context, rm.ProviderDealIdentifier, datatransfer.ChannelID) error
	HandleDisconnectFor(context.Context, rm.ProviderDealIdentifier, error) error
	HandlePaymentRequested(context.Context, rm.ProviderDealIdentifier) error
	HandleProcessPayment(context.Context, rm.ProviderDealIdentifier) error
	HandleLastPayment(context.Context, rm.ProviderDealIdentifier) error

	HandleCancelForDeal(context.Context, rm.ProviderDealIdentifier) error
	HandleErrorForDeal(context.Context, rm.ProviderDealIdentifier, error) error
	TryHandleCompleted(context.Context, rm.ProviderDealIdentifier) error
}

var _ IDatatransferHandler = (*DataTransferHandler)(nil)

type DataTransferHandler struct {
	retrievalDealHandler IRetrievalHandler
	retrievalDealStore   repo.IRetrievalDealRepo
}

func NewDataTransferHandler(retrievalDealHandler IRetrievalHandler, retrievalDealStore repo.IRetrievalDealRepo) *DataTransferHandler {
	return &DataTransferHandler{retrievalDealHandler: retrievalDealHandler, retrievalDealStore: retrievalDealStore}
}

func (d *DataTransferHandler) HandleCompleteFor(ctx context.Context, identifier rm.ProviderDealIdentifier) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	return d.retrievalDealHandler.CleanupDeal(ctx, deal)
}

func (d *DataTransferHandler) TryHandleCompleted(ctx context.Context, identifier rm.ProviderDealIdentifier) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	if deal.Status == rm.DealStatusFinalizing {
		return d.retrievalDealHandler.CleanupDeal(ctx, deal)
	}
	return nil
}

func (d *DataTransferHandler) HandleAcceptFor(ctx context.Context, identifier rm.ProviderDealIdentifier, channelId datatransfer.ChannelID) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	// state transition should follow `ProviderEventDealAccepted` event
	// https://github.com/filecoin-project/go-fil-markets/blob/9e5f2499cba68968ffc75a22b89a085c5722f1a5/retrievalmarket/impl/providerstates/provider_fsm.go#L32-L38
	deal.ChannelID = &channelId
	if err = d.retrievalDealStore.SaveDeal(ctx, deal); err != nil {
		return err
	}

	switch deal.Status {
	case rm.DealStatusFundsNeededUnseal: // nothing needs to do.
		return nil
	case rm.DealStatusNew:
		err := d.retrievalDealHandler.UnsealData(ctx, deal)
		if err != nil {
			log.Errorf("unseal data error: %s", err.Error())
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid state transition, state `%+v`, event `%+v`", deal.Status, rm.ProviderEventDealAccepted)
	}
}

func (d *DataTransferHandler) HandleDisconnectFor(ctx context.Context, identifier rm.ProviderDealIdentifier, errIn error) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	return d.retrievalDealHandler.Error(ctx, deal, errIn)
}

func (d *DataTransferHandler) HandlePaymentRequested(ctx context.Context, identifier rm.ProviderDealIdentifier) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	if deal.Status == rm.DealStatusOngoing || deal.Status == rm.DealStatusUnsealed {
		deal.Status = rm.DealStatusFundsNeeded
		if err := d.retrievalDealStore.SaveDeal(ctx, deal); err != nil {
			return err
		}
	}
	if deal.Status == rm.DealStatusNew {
		deal.Status = rm.DealStatusFundsNeededUnseal
		if err := d.retrievalDealStore.SaveDeal(ctx, deal); err != nil {
			return err
		}
	}
	return d.retrievalDealHandler.UpdateFunding(ctx, deal)
}

func (d *DataTransferHandler) HandleProcessPayment(ctx context.Context, identifier rm.ProviderDealIdentifier) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	return d.retrievalDealHandler.UpdateFunding(ctx, deal)
}

func (d *DataTransferHandler) HandleLastPayment(ctx context.Context, identifier rm.ProviderDealIdentifier) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	if deal.Status == rm.DealStatusUnsealed || deal.Status == rm.DealStatusOngoing {
		deal.Status = rm.DealStatusFundsNeededLastPayment
		if err := d.retrievalDealStore.SaveDeal(ctx, deal); err != nil {
			return err
		}
		return d.retrievalDealHandler.UpdateFunding(ctx, deal)
	}
	return nil
}

func (d *DataTransferHandler) HandleCancelForDeal(ctx context.Context, identifier rm.ProviderDealIdentifier) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	switch deal.Status {
	case rm.DealStatusFailing:
	case rm.DealStatusCancelling:
	default:
		if deal.Status != rm.DealStatusFailing {
			deal.Message = "Client cancelled retrieval"
		}
		if err := d.retrievalDealHandler.CancelDeal(ctx, deal); err != nil {
			return err
		}
	}
	return nil
}

func (d *DataTransferHandler) HandleErrorForDeal(ctx context.Context, identifier rm.ProviderDealIdentifier, errIn error) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	return d.retrievalDealHandler.Error(ctx, deal, errIn)
}
