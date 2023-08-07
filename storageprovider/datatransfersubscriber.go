package storageprovider

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
)

// EventReceiver is any thing that can receive FSM events
type IDatatransferHandler interface {
	// have many receiver function
	HandleCompleteFor(ctx context.Context, proposalid cid.Cid) error
	HandleCancelForDeal(ctx context.Context, proposalid cid.Cid) error
	HandleRestartForDeal(ctx context.Context, proposalid cid.Cid, channelID datatransfer.ChannelID) error
	HandleStalledForDeal(ctx context.Context, proposalid cid.Cid) error
	HandleInitForDeal(ctx context.Context, proposalid cid.Cid, channel datatransfer.ChannelID) error
	HandleFailedForDeal(ctx context.Context, proposalid cid.Cid, reason error) error
}

// ProviderDataTransferSubscriber is the function called when an event occurs in a data
// transfer received by a provider -- it reads the voucher to verify this event occurred
// in a storage market deal, then, based on the data transfer event that occurred, it generates
// and update message for the deal -- either moving to staged for a completion
// event or moving to error if a data transfer error occurs
func ProviderDataTransferSubscriber(deals IDatatransferHandler, eventPublisher *EventPublishAdapter) datatransfer.Subscriber {
	return func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		node := channelState.Voucher()
		if node.Voucher == nil {
			log.Debugw("ignoring data-transfer event as it's not storage related", "event", datatransfer.Events[event.Code], "channelID",
				channelState.ChannelID())
			return
		}
		voucherIface, err := requestvalidation.BindnodeRegistry.TypeFromNode(node.Voucher, &requestvalidation.StorageDataTransferVoucher{})
		// if this event is for a transfer not related to storage, ignore
		if err != nil {
			log.Debugw("ignoring data-transfer event as it's not storage related", "event", datatransfer.Events[event.Code], "channelID",
				channelState.ChannelID())
			return
		}
		voucher, _ := voucherIface.(*requestvalidation.StorageDataTransferVoucher) // safe to assume type

		log.Debugw("processing storage provider dt event", "event", datatransfer.Events[event.Code], "proposalCid", voucher.Proposal, "channelID",
			channelState.ChannelID(), "channelState", datatransfer.Statuses[channelState.Status()])

		ctx := context.TODO()
		log.Debugw("processing storage provider dt event", "event", datatransfer.Events[event.Code], "proposalCid", voucher.Proposal, "channelID",
			channelState.ChannelID(), "channelState", datatransfer.Statuses[channelState.Status()])

		if channelState.Status() == datatransfer.Completed {
			// on complete
			eventPublisher.PublishWithCid(storagemarket.ProviderEventDataTransferCompleted, voucher.Proposal)
			err := deals.HandleCompleteFor(ctx, voucher.Proposal)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		}

		// Translate from data transfer events to provider FSM events
		// Note: We ignore data transfer progress events (they do not affect deal state)
		err = func() error {
			switch event.Code {
			case datatransfer.Cancel:
				eventPublisher.PublishWithCid(storagemarket.ProviderEventDataTransferCancelled, voucher.Proposal)
				return deals.HandleCancelForDeal(ctx, voucher.Proposal)
			case datatransfer.Restart:
				eventPublisher.PublishWithCid(storagemarket.ProviderEventDataTransferRestarted, voucher.Proposal)
				return deals.HandleRestartForDeal(ctx, voucher.Proposal, channelState.ChannelID())
			case datatransfer.Disconnected:
				eventPublisher.PublishWithCid(storagemarket.ProviderEventDataTransferStalled, voucher.Proposal)
				return deals.HandleStalledForDeal(ctx, voucher.Proposal)
			case datatransfer.Open:
				eventPublisher.PublishWithCid(storagemarket.ProviderEventDataTransferInitiated, voucher.Proposal)
				return deals.HandleInitForDeal(ctx, voucher.Proposal, channelState.ChannelID())
			case datatransfer.Error:
				eventPublisher.PublishWithCid(storagemarket.ProviderEventDataTransferFailed, voucher.Proposal)
				return deals.HandleFailedForDeal(ctx, voucher.Proposal, fmt.Errorf("deal data transfer failed: %s", event.Message))
			case datatransfer.DataQueued:
				eventPublisher.PublishWithCid(storagemarket.ProviderEventDataRequested, voucher.Proposal)
				return nil
			default:
				return nil
			}
		}()
		if err != nil {
			log.Errorw("error processing storage provider dt event", "event", datatransfer.Events[event.Code], "proposalCid", voucher.Proposal, "channelID",
				channelState.ChannelID(), "err", err)
		}
	}
}
