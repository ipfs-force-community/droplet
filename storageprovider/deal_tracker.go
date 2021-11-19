package storageprovider

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"go.uber.org/fx"

	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus/app/client/apiface"

	vTypes "github.com/filecoin-project/venus/pkg/types"

	"github.com/ipfs-force-community/venus-common-utils/metrics"
)

type DealTracker struct {
	period      time.Duration // TODO: Preferably configurable?
	storageRepo repo.StorageDealRepo
	minerMgr    minermgr.IMinerMgr
	fullNode    apiface.FullNode
}

var ReadyRetrievalDealStatus = []storagemarket.StorageDealStatus{storagemarket.StorageDealAwaitingPreCommit, storagemarket.StorageDealSealing, storagemarket.StorageDealActive}

func NewDealTracker(lc fx.Lifecycle, r repo.Repo, minerMgr minermgr.IMinerMgr, fullNode apiface.FullNode) *DealTracker {
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
		dealTracker.checkCommit(ctx, addr, head.Key())
		dealTracker.checkPreCommit(ctx, addr, head.Key())
		//todo check expire
	}
}

func (dealTracker *DealTracker) checkPreCommit(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(addr, storagemarket.StorageDealAwaitingPreCommit)
	if err != nil {
		log.Errorf("get miner %s storage deals for check StorageDealAwaitingPreCommit %w", addr, err)
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
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(addr, storagemarket.StorageDealSealing)
	if err != nil {
		log.Errorf("get miner %s storage deals for check StorageDealSealing %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, tsk)
		if err != nil {
			log.Errorf("get market deal for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		if dealProposal.State.SectorStartEpoch > -1 { //include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(deal.ProposalCid, storagemarket.StorageDealActive)
			if err != nil {
				log.Errorf("update deal status to active for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}
	}
}

func (dealTracker *DealTracker) checkSlash(ctx metrics.MetricsCtx, addr address.Address, tsk vTypes.TipSetKey) {
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(addr, storagemarket.StorageDealActive)
	if err != nil {
		log.Errorf("get miner %s storage deals for check StorageDealActive %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, tsk)
		if err != nil {
			log.Errorf("get market deal info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		if dealProposal.State.SlashEpoch > -1 { //include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(deal.ProposalCid, storagemarket.StorageDealSlashed)
			if err != nil {
				log.Errorf("update deal status to slash for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}
	}
}
