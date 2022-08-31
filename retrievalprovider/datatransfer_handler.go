package retrievalprovider

import (
	"context"
	"fmt"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus-market/v2/models/repo"
)

// EventReceiver is any thing that can receive FSM events
type IDatatransferHandler interface {
	//have many receiver function
	HandleCompleteFor(context.Context, rm.ProviderDealIdentifier) error
	HandleAcceptFor(context.Context, rm.ProviderDealIdentifier, datatransfer.ChannelID) error
	HandleDisconnectFor(context.Context, rm.ProviderDealIdentifier, error) error

	HandleCancelForDeal(context.Context, rm.ProviderDealIdentifier) error
	HandleErrorForDeal(context.Context, rm.ProviderDealIdentifier, error) error
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
		return d.retrievalDealHandler.UnsealData(ctx, deal)
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
	}
	return d.retrievalDealStore.SaveDeal(ctx, deal)
}

func (d *DataTransferHandler) HandleErrorForDeal(ctx context.Context, identifier rm.ProviderDealIdentifier, errIn error) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	return d.retrievalDealHandler.Error(ctx, deal, errIn)
}
