package client

import (
	"context"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-auth/log"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
	"go.uber.org/fx"
)

type DealTracker struct {
	full       v1api.FullNode
	dealRepo   repo.ClientOfflineDealRepo
	dealClient storagemarket.StorageClient
}

func NewDealTracker(lc fx.Lifecycle,
	full v1api.FullNode,
	offlineDealRepo repo.ClientOfflineDealRepo,
	dealClient storagemarket.StorageClient,
) *DealTracker {
	dt := &DealTracker{
		full:       full,
		dealRepo:   offlineDealRepo,
		dealClient: dealClient,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go dt.loopRefreshDealState(ctx)

			return nil
		},
	})

	return dt
}

func (dt *DealTracker) loadDeals(ctx context.Context) ([]*types.ClientOfflineDeal, []*types.ClientOfflineDeal, error) {
	deals, err := dt.dealRepo.ListDeal(ctx)
	if err != nil {
		return nil, nil, err
	}
	var activeDeals, inactiveDeals []*types.ClientOfflineDeal

	for _, deal := range deals {
		if storageprovider.IsTerminateState(deal.State) {
			continue
		}
		if deal.State != storagemarket.StorageDealActive {
			inactiveDeals = append(inactiveDeals, deal)
		} else {
			activeDeals = append(activeDeals, deal)
		}
	}

	return activeDeals, inactiveDeals, nil
}

func (dt *DealTracker) loopRefreshDealState(ctx context.Context) {
	dt.refreshDealState(ctx)
	dt.checkSlash(ctx)

	ticker := time.NewTicker(time.Minute * 3)
	defer ticker.Stop()

	slashTicker := time.NewTimer(time.Hour * 6)
	defer slashTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dt.refreshDealState(ctx)
		case <-slashTicker.C:
			dt.checkSlash(ctx)
		}
	}
}

func (dt *DealTracker) refreshDealState(ctx context.Context) {
	_, inactiveDeals, err := dt.loadDeals(ctx)
	if err != nil {
		log.Warnf("load offline deal failed: %v", err)
		return
	}

	for _, deal := range inactiveDeals {
		proposalCID := deal.ProposalCID
		status, err := dt.dealClient.GetProviderDealState(ctx, proposalCID)
		if err != nil {
			log.Warnf("failed to got deal status: %v %v", proposalCID, err)
			continue
		}
		var needUpdate bool
		if deal.State != status.State {
			deal.State = status.State
			needUpdate = true
		}
		if deal.Message != status.Message {
			deal.Message = status.Message
			needUpdate = true
		}
		if deal.DealID != uint64(status.DealID) {
			deal.DealID = uint64(status.DealID)
			needUpdate = true
		}
		if status.AddFundsCid != nil {
			deal.AddFundsCid = status.AddFundsCid
			needUpdate = true
		}
		if status.PublishCid != nil {
			deal.PublishMessage = status.PublishCid
			needUpdate = true
		}
		if needUpdate {
			dt.persistDeal(ctx, deal)
		}
	}
}

func (dt *DealTracker) checkSlash(ctx context.Context) {
	deals, _, err := dt.loadDeals(ctx)
	if err != nil {
		log.Warnf("load offline deal failed: %v", err)
		return
	}
	for i, deal := range deals {
		md, err := dt.full.StateMarketStorageDeal(ctx, abi.DealID(deal.DealID), shared.EmptyTSK)
		if err == nil && md.State.SlashEpoch > -1 {
			deals[i].State = storagemarket.StorageDealSlashed
			dt.persistDeal(ctx, deals[i])
		}
	}
}

func (dt *DealTracker) persistDeal(ctx context.Context, deal *types.ClientOfflineDeal) {
	deal.UpdatedAt = time.Now()
	if err := dt.dealRepo.SaveDeal(ctx, deal); err != nil {
		log.Errorf("failed to save deal: %s %v", deal.ProposalCID, err)
	}
}
