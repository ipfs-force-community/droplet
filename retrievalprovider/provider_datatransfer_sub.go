package retrievalprovider

import (
	"context"
	"fmt"

	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

// ProviderDataTransferSubscriber is the function called when an event occurs in a data
// transfer received by a provider -- it reads the voucher to verify this event occurred
// in a storage market deal, then, based on the data transfer event that occurred, it generates
// and update message for the deal -- either moving to staged for a completion
// event or moving to error if a data transfer error occurs
func ProviderDataTransferSubscriber(deals IDatatransferHandler) datatransfer.Subscriber {
	return func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		ctx := context.TODO()
		voucher := channelState.Voucher()
		if voucher.Voucher == nil {
			log.Errorf("received empty voucher")
			return
		}
		dealProposal, err := rm.DealProposalFromNode(voucher.Voucher)
		// if this event is for a transfer not related to storage, ignore
		if err != nil {
			return
		}

		mlog := log.With("event", datatransfer.Events[event.Code], "dealID", dealProposal.ID, "peer", channelState.OtherPeer())

		identify := rm.ProviderDealIdentifier{DealID: dealProposal.ID, Receiver: channelState.Recipient()}
		if channelState.Status() == datatransfer.Completed {
			mlog.Debugf("deal completed")
			err := deals.HandleCompleteFor(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		}

		switch event.Code {
		case datatransfer.Accept:
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventDealAccepted])
			err := deals.HandleAcceptFor(ctx, identify, channelState.ChannelID())
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.Disconnected:
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventDataTransferError])
			err := deals.HandleDisconnectFor(ctx, identify, fmt.Errorf("deal data transfer stalled (peer hungup)"))
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.Error:
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventDataTransferError])
			err := deals.HandleErrorForDeal(ctx, identify, fmt.Errorf("deal data transfer failed: %s", event.Message))
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.DataLimitExceeded:
			// DataLimitExceeded indicates it's time to wait for a payment
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventPaymentRequested])
			err := deals.HandlePaymentRequested(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.BeginFinalizing:
			// BeginFinalizing indicates it's time to wait for a final payment
			// Because the legacy client expects a final voucher, we dispatch this event event when
			// the deal is free -- so that we have a chance to send this final voucher before completion
			// TODO: do not send the legacy voucher when the client no longer expects it
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventLastPaymentRequested])
			err := deals.HandleLastPayment(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.NewVoucher:
			// NewVoucher indicates a potential new payment we should attempt to process
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventProcessPayment])
			err := deals.HandleProcessPayment(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.Cancel:
			mlog = mlog.With("retrievalEvent", rm.ProviderEvents[rm.ProviderEventClientCancelled])
			err := deals.HandleCancelForDeal(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.NewVoucherResult:
			mlog = mlog.With("channelStatus", channelState.Status())
			if channelState.Status() == datatransfer.Finalizing {
				err := deals.HandleCompleteFor(ctx, identify)
				if err != nil {
					log.Errorf("processing dt event: %s", err)
				}
			}
		default:
			return
		}
		mlog.Debugw("processing retrieval provider dt event")
	}
}
