package storageprovider

import (
	"context"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus-market/models/repo"
)

var _ IDatatransferHandler = (*DataTransferHandler)(nil)

type DataTransferHandler struct {
	dealProcess StorageDealHandler
	deals       repo.StorageDealRepo
}

func NewDataTransferProcess(
	dealProcess StorageDealHandler,
	deals repo.StorageDealRepo,
) IDatatransferHandler {
	return &DataTransferHandler{
		dealProcess: dealProcess,
		deals:       deals,
	}
}

func (d *DataTransferHandler) HandleCompleteFor(proposalid cid.Cid) error {
	//should never failed
	ctx := context.TODO()
	deal, err := d.deals.GetDeal(proposalid)
	if err != nil {
		return xerrors.Errorf("get deal while transfer completed %w", err)
	}
	deal.State = storagemarket.StorageDealVerifyData
	err = d.deals.SaveDeal(deal)
	if err != nil {
		return xerrors.Errorf("save deal while transfer completed %w", err)
	}
	go d.dealProcess.HandleOff(ctx, deal) //nolint
	return nil
}

func (d *DataTransferHandler) HandleCancelForDeal(proposalid cid.Cid) error {
	deal, err := d.deals.GetDeal(proposalid)
	if err != nil {
		return xerrors.Errorf("get deal while transfer completed %w", err)
	}
	return d.dealProcess.HandleError(deal, xerrors.Errorf("proposal %v data transfer cancelled", proposalid))
}

func (d *DataTransferHandler) HandleRestartForDeal(proposalid cid.Cid, channelId datatransfer.ChannelID) error {
	deal, err := d.deals.GetDeal(proposalid)
	if err != nil {
		return xerrors.Errorf("get deal while transfer completed %w", err)
	}
	deal.Message = ""
	deal.State = storagemarket.StorageDealProviderTransferAwaitRestart
	deal.TransferChannelId = &channelId
	err = d.deals.SaveDeal(deal)
	if err != nil {
		return xerrors.Errorf("save deal while transfer completed %w", err)
	}
	return nil
}

func (d *DataTransferHandler) HandleStalledForDeal(proposalid cid.Cid) error {
	deal, err := d.deals.GetDeal(proposalid)
	if err != nil {
		return xerrors.Errorf("get deal while transfer completed %w", err)
	}
	deal.Message = "data transfer appears to be stalled, awaiting reconnect from client"
	deal.State = storagemarket.StorageDealProviderTransferAwaitRestart
	err = d.deals.SaveDeal(deal)
	if err != nil {
		return xerrors.Errorf("save deal while transfer completed %w", err)
	}
	return nil
}

func (d *DataTransferHandler) HandleInitForDeal(proposalid cid.Cid, channelId datatransfer.ChannelID) error {
	deal, err := d.deals.GetDeal(proposalid)
	if err != nil {
		return xerrors.Errorf("get deal while transfer completed %w", err)
	}
	deal.Message = ""
	deal.State = storagemarket.StorageDealProviderTransferAwaitRestart
	deal.TransferChannelId = &channelId
	err = d.deals.SaveDeal(deal)
	if err != nil {
		return xerrors.Errorf("save deal while transfer completed %w", err)
	}
	return nil
}

func (d *DataTransferHandler) HandleFailedForDeal(proposalid cid.Cid, reason error) error {
	deal, err := d.deals.GetDeal(proposalid)
	if err != nil {
		return xerrors.Errorf("get deal while transfer completed %w", err)
	}
	deal.Message = xerrors.Errorf("error transferring data: %w", reason).Error()
	deal.State = storagemarket.StorageDealProviderTransferAwaitRestart
	err = d.deals.SaveDeal(deal)
	if err != nil {
		return xerrors.Errorf("save deal while transfer completed %w", err)
	}
	return nil
}
