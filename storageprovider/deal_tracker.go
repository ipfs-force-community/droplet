package storageprovider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"go.uber.org/fx"

	"github.com/filecoin-project/venus-market/v2/minermgr"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"

	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/ipfs-force-community/metrics"
)

type DealTracker struct {
	period      time.Duration // TODO: Preferably configurable?
	storageRepo repo.StorageDealRepo
	minerMgr    minermgr.IAddrMgr
	fullNode    v1api.FullNode
}

var ReadyRetrievalDealStatus = []storagemarket.StorageDealStatus{storagemarket.StorageDealAwaitingPreCommit, storagemarket.StorageDealSealing, storagemarket.StorageDealActive}

func NewDealTracker(lc fx.Lifecycle, r repo.Repo, minerMgr minermgr.IAddrMgr, fullNode v1api.FullNode) *DealTracker {
	tracker := &DealTracker{period: time.Minute, storageRepo: r.StorageDealRepo(), minerMgr: minerMgr, fullNode: fullNode}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go tracker.Start(ctx)
			return nil
		},
	})
	return tracker
}

func (dealTracker *DealTracker) Start(ctx metrics.MetricsCtx) {
	dealTracker.scanDeal(ctx)
	ticker := time.NewTicker(dealTracker.period)
	for {
		select {
		case <-ticker.C:
			dealTracker.scanDeal(ctx)
		case <-ctx.Done():
			log.Warnf("exit deal tracker by context")
			return
		}
	}
}

func (dealTracker *DealTracker) scanDeal(ctx metrics.MetricsCtx) {
	addrs, err := dealTracker.minerMgr.ActorAddress(ctx)
	if err != nil {
		log.Errorf("get miners list %w", err)
	}
	head, err := dealTracker.fullNode.ChainHead(ctx)
	if err != nil {
		log.Errorf("get chain head %w", err)
	}

	for _, addr := range addrs {
		dealTracker.checkSlash(ctx, addr, head.Key())
		dealTracker.checkPreCommitAndCommit(ctx, addr, head.Key())
		//todo check expire
	}
}

func (dealTracker *DealTracker) checkPreCommitAndCommit(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(ctx, addr, storagemarket.StorageDealAwaitingPreCommit, storagemarket.StorageDealSealing)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		log.Errorf("get miner %s storage deals for check StorageDealAwaitingPreCommit %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, tsk)
		if err != nil {
			//todo if deal not found maybe need to market storage deal as error
			log.Errorf("get market deal for sector %d of miner %s %s", deal.SectorNumber, addr, err.Error())
			continue
		}
		if dealProposal.State.SectorStartEpoch > -1 { //include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(ctx, deal.ProposalCid, storagemarket.StorageDealActive, market.Proving)
			if err != nil {
				log.Errorf("update deal status to active for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
			continue
		}

		if deal.State == storagemarket.StorageDealAwaitingPreCommit {
			preInfo, err := dealTracker.fullNode.StateSectorPreCommitInfo(ctx, addr, deal.SectorNumber, tsk)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					continue
				}
				log.Debugf("get precommit info for sector %d of miner %s: %s", deal.SectorNumber, addr, err)
				continue
			}

			dealExist := false
			for _, dealID := range preInfo.Info.DealIDs {
				if dealID == deal.DealID {
					dealExist = true
					break
				}
			}
			if !dealExist {
				log.Warnf("deal %d does not exist in sector %d of miner %s", deal.DealID, deal.SectorNumber, addr)
				continue
			}

			err = dealTracker.storageRepo.UpdateDealStatus(ctx, deal.ProposalCid, storagemarket.StorageDealSealing, market.Packing)
			if err != nil {
				log.Errorf("update deal status to sealing for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}

		//todo may skip storage dealsealing, and run into active
	}
}

func (dealTracker *DealTracker) checkSlash(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(ctx, addr, storagemarket.StorageDealActive)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		log.Errorf("get miner %s storage deals for check StorageDealActive %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, tsk)
		if err != nil {
			log.Errorf("get market deal info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			continue
		}
		if dealProposal.State.SlashEpoch > -1 { //include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(ctx, deal.ProposalCid, storagemarket.StorageDealSlashed, "")
			if err != nil {
				log.Errorf("update deal status to slash for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}
	}
}
