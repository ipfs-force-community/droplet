package storageprovider

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus/app/client/apiface"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"go.uber.org/fx"
	"time"
)

type DealTracker struct {
	period      time.Duration
	storageRepo repo.StorageDealRepo
	minerMgr    minermgr.IMinerMgr
	fullNode    apiface.FullNode
}

var ReadyRetrievalDealStatus = []storagemarket.StorageDealStatus{storagemarket.StorageDealAwaitingPreCommit, storagemarket.StorageDealSealing, storagemarket.StorageDealActive}

func NewDealTracker(lc fx.Lifecycle, r repo.Repo, minerMgr minermgr.IMinerMgr, fullNode apiface.FullNode) *DealTracker {
	tracker := &DealTracker{storageRepo: r.StorageDealRepo(), minerMgr: minerMgr, fullNode: fullNode}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			tracker.Start(ctx)
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
		dealTracker.checkCommit(ctx, addr, head.Key())
		dealTracker.checkPrecommit(ctx, addr, head.Key())
		//todo check expire
	}
}

func (dealTracker *DealTracker) checkPrecommit(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealbyAddrAndStatus(addr, storagemarket.StorageDealAwaitingPreCommit)
	if err != nil {
		log.Errorf("get miner %s storage deals for check precommit %w", addr, err)
	}

	for _, deal := range deals {
		_, err := dealTracker.fullNode.StateSectorPreCommitInfo(ctx, addr, deal.SectorNumber, tsk)
		if err != nil {
			log.Errorf("get precommit info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		err = dealTracker.storageRepo.UpdateDealStatus(deal.ProposalCid, storagemarket.StorageDealSealing)
		if err != nil {
			log.Errorf("update deal status to sealing for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
	}
}

func (dealTracker *DealTracker) checkCommit(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealbyAddrAndStatus(addr, storagemarket.StorageDealSealing)
	if err != nil {
		log.Errorf("get miner %s storage deals for check precommit %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, tsk)
		if err != nil {
			log.Errorf("get precommit info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		if dealProposal.State.SectorStartEpoch > -1 { //include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(deal.ProposalCid, storagemarket.StorageDealActive)
			if err != nil {
				log.Errorf("update deal status to sealing for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}
	}
}

func (dealTracker *DealTracker) checkSlash(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealbyAddrAndStatus(addr, storagemarket.StorageDealActive)
	if err != nil {
		log.Errorf("get miner %s storage deals for check precommit %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, tsk)
		if err != nil {
			log.Errorf("get precommit info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		if dealProposal.State.SlashEpoch > -1 { //include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(deal.ProposalCid, storagemarket.StorageDealSlashed)
			if err != nil {
				log.Errorf("update deal status to sealing for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}
	}
}
