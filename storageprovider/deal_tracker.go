package storageprovider

import (
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus/app/client/apiface"
	"time"
)

type DealTracker struct {
	period      time.Duration
	storageRepo repo.StorageDealRepo
	minerMgr    minermgr.IMinerMgr
	fullNode    apiface.FullNode
}

func (dealTracker *DealTracker) Start(ctx metrics.MetricsCtx) {
	ticker := time.NewTicker(dealTracker.period)

	for {
		select {
		case <-ticker.C:

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
		deals, err := dealTracker.storageRepo.GetDealbyAddrAndStatus(addr, storagemarket.StorageDealAwaitingPreCommit)
		if err != nil {
			log.Errorf("get miner %s storage deals for check precommit %w", addr, err)
		}

		for _, deal := range deals {
			_, err := dealTracker.fullNode.StateSectorPreCommitInfo(ctx, addr, deal.SectorNumber, head.Key())
			if err != nil {
				log.Errorf("get precommit info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
			err = dealTracker.storageRepo.UpdateDealStatus(deal.ProposalCid, storagemarket.StorageDealSealing)
			if err != nil {
				log.Errorf("update deal status to sealing for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}
		}
	}

}
