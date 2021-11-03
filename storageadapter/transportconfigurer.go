package storageadapter

import (
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"

	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/types"
)

type providerStoreGetter struct {
	stores *stores.ReadWriteBlockstores
	deals  repo.StorageDealRepo
}

func newProviderStoreGetter(deals repo.StorageDealRepo, stores *stores.ReadWriteBlockstores) *providerStoreGetter {
	return &providerStoreGetter{
		deals:  deals,
		stores: stores,
	}
}

func (psg *providerStoreGetter) Get(proposalCid cid.Cid) (bstore.Blockstore, error) {
	deal, err := psg.deals.GetDeal(proposalCid)
	if err != nil {
		return nil, xerrors.Errorf("failed to get deal state: %w", err)
	}
	return psg.stores.GetOrOpen(proposalCid.String(), deal.InboundCAR, deal.Ref.Root)
}

type providerPushDeals struct {
	deals repo.StorageDealRepo
}

func convertMinerDealToFilMarketDeal(deal *types.MinerDeal) *storagemarket.MinerDeal {
	return &storagemarket.MinerDeal{
		ClientDealProposal:    deal.ClientDealProposal,
		ProposalCid:           deal.ProposalCid,
		AddFundsCid:           deal.AddFundsCid,
		PublishCid:            deal.PublishCid,
		Miner:                 deal.Miner,
		Client:                deal.Client,
		State:                 deal.State,
		PiecePath:             deal.PiecePath,
		MetadataPath:          deal.MetadataPath,
		SlashEpoch:            deal.SlashEpoch,
		FastRetrieval:         deal.FastRetrieval,
		Message:               deal.Message,
		FundsReserved:         deal.FundsReserved,
		Ref:                   deal.Ref,
		AvailableForRetrieval: deal.AvailableForRetrieval,

		DealID:       deal.DealID,
		CreationTime: deal.CreationTime,

		TransferChannelId: deal.TransferChannelId,
		SectorNumber:      deal.SectorNumber,

		InboundCAR: deal.InboundCAR,
	}
}

func (ppd *providerPushDeals) Get(proposalCid cid.Cid) (storagemarket.MinerDeal, error) {
	deal, err := ppd.deals.GetDeal(proposalCid)
	if err != nil {
		return storagemarket.MinerDeal{}, err
	}
	return *convertMinerDealToFilMarketDeal(deal), nil
}
