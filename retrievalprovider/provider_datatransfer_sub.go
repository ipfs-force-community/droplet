package retrievalprovider

import (
	"context"
	"fmt"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/migrations"
)

// ProviderDataTransferSubscriber is the function called when an event occurs in a data
// transfer received by a provider -- it reads the voucher to verify this event occurred
// in a storage market deal, then, based on the data transfer event that occurred, it generates
// and update message for the deal -- either moving to staged for a completion
// event or moving to error if a data transfer error occurs
func ProviderDataTransferSubscriber(deals IDatatransferHandler) datatransfer.Subscriber {
	return func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		ctx := context.TODO()
		dealProposal, ok := dealProposalFromVoucher(channelState.Voucher())
		// if this event is for a transfer not related to storage, ignore
		if !ok {
			return
		}

		identify := rm.ProviderDealIdentifier{DealID: dealProposal.ID, Receiver: channelState.Recipient()}
		if channelState.Status() == datatransfer.Completed {
			err := deals.HandleCompleteFor(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		}

		mlog := log.With("event", datatransfer.Events[event.Code], "dealID", dealProposal.ID, "peer", channelState.OtherPeer())
		switch event.Code {
		case datatransfer.Accept:
			mlog.With("retrievalEvent", rm.ProviderEventDealAccepted)
			err := deals.HandleAcceptFor(ctx, identify, channelState.ChannelID())
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.Disconnected:
			mlog.With("retrievalEvent", rm.ProviderEventDataTransferError)
			err := deals.HandleDisconnectFor(ctx, identify, fmt.Errorf("deal data transfer stalled (peer hungup)"))
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.Error:
			mlog.With("retrievalEvent", rm.ProviderEventDataTransferError)
			err := deals.HandleErrorForDeal(ctx, identify, fmt.Errorf("deal data transfer failed: %s", event.Message))
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		case datatransfer.Cancel:
			mlog.With("retrievalEvent", rm.ProviderEventClientCancelled)
			err := deals.HandleCancelForDeal(ctx, identify)
			if err != nil {
				log.Errorf("processing dt event: %s", err)
			}
		default:
			return
		}
		mlog.Debugw("processing retrieval provider dt event")
	}
}

func dealProposalFromVoucher(voucher datatransfer.Voucher) (*rm.DealProposal, bool) {
	dealProposal, ok := voucher.(*rm.DealProposal)
	// if this event is for a transfer not related to storage, ignore
	if ok {
		return dealProposal, true
	}

	legacyProposal, ok := voucher.(*migrations.DealProposal0)
	if !ok {
		return nil, false
	}
	newProposal := migrations.MigrateDealProposal0To1(*legacyProposal)
	return &newProposal, true
}
