package retrievalprovider

import (
	"context"

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
	return d.retrievalDealHandler.CleanupDeal(context.TODO(), deal)
}

func (d *DataTransferHandler) HandleAcceptFor(ctx context.Context, identifier rm.ProviderDealIdentifier, channelId datatransfer.ChannelID) error {
	deal, err := d.retrievalDealStore.GetDeal(ctx, identifier.Receiver, identifier.DealID)
	if err != nil {
		return err
	}
	deal.ChannelID = &channelId
	return d.retrievalDealHandler.UnsealData(ctx, deal)
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
