package storageprovider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/ipfs-force-community/droplet/v2/indexprovider"
	"github.com/ipfs-force-community/droplet/v2/minermgr"
	"github.com/ipfs-force-community/droplet/v2/models/repo"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/ipfs-force-community/metrics"
)

var globalRand *rand.Rand

func init() {
	globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))
}

type DealTracker struct {
	storageRepo      repo.StorageDealRepo
	minerMgr         minermgr.IMinerMgr
	fullNode         v1api.FullNode
	eventPublisher   *EventPublishAdapter
	indexProviderMgr *indexprovider.IndexProviderMgr
}

var ReadyRetrievalDealStatus = []storagemarket.StorageDealStatus{storagemarket.StorageDealAwaitingPreCommit, storagemarket.StorageDealSealing, storagemarket.StorageDealActive}

func NewDealTracker(lc fx.Lifecycle,
	r repo.Repo,
	minerMgr minermgr.IMinerMgr,
	fullNode v1api.FullNode,
	pb *EventPublishAdapter,
	indexProviderMgr *indexprovider.IndexProviderMgr,
) *DealTracker {
	tracker := &DealTracker{
		storageRepo:      r.StorageDealRepo(),
		minerMgr:         minerMgr,
		fullNode:         fullNode,
		eventPublisher:   pb,
		indexProviderMgr: indexProviderMgr,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go tracker.Start(ctx)
			return nil
		},
	})
	return tracker
}

func (dealTracker *DealTracker) Start(ctx metrics.MetricsCtx) {
	ticker := time.NewTicker(time.Minute*10 + time.Minute*time.Duration(globalRand.Intn(10)))
	defer ticker.Stop()

	slashTicker := time.NewTicker(time.Hour*3 + time.Minute*time.Duration(globalRand.Intn(30)))
	defer slashTicker.Stop()

	dealTracker.scanDeal(ctx, true, true)

	for {
		select {
		case <-ticker.C:
			dealTracker.scanDeal(ctx, true, false)
		case <-slashTicker.C:
			dealTracker.scanDeal(ctx, false, true)
		case <-ctx.Done():
			log.Warnf("exit deal tracker by context")
			return
		}
	}
}

func (dealTracker *DealTracker) scanDeal(ctx metrics.MetricsCtx, checkPreCommitAndCommit, checkSlash bool) {
	actors, err := dealTracker.minerMgr.ActorList(ctx)
	if err != nil {
		log.Errorf("get actor list err: %s", err)
	}
	head, err := dealTracker.fullNode.ChainHead(ctx)
	if err != nil {
		log.Errorf("get chain head err: %s", err)
	}

	for _, actor := range actors {
		if checkSlash {
			err = dealTracker.checkSlash(ctx, actor.Addr, head)
			if err != nil {
				log.Errorf("check slash err: %s", err)
			}
		}

		if checkPreCommitAndCommit {
			err = dealTracker.checkPreCommitAndCommit(ctx, actor.Addr, head)
			if err != nil {
				log.Errorf("check precommit/commit expired err: %s", err)
			}
		}
	}
}

func (dealTracker *DealTracker) checkPreCommitAndCommit(ctx metrics.MetricsCtx, addr address.Address, ts *vTypes.TipSet) error {
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(ctx, addr, storagemarket.StorageDealAwaitingPreCommit, storagemarket.StorageDealSealing)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return fmt.Errorf("get miner %s storage deals for check StorageDealAwaitingPreCommit %w", addr, err)
	}

	curHeight := ts.Height()

	for _, deal := range deals {
		if deal.Proposal.StartEpoch < curHeight {
			err = dealTracker.storageRepo.UpdateDealStatus(ctx, deal.ProposalCid, storagemarket.StorageDealExpired, "")
			if err != nil {
				return fmt.Errorf("update deal %d status to of miner %s expired %w", deal.DealID, addr, err)
			}
			log.Infof("update deal %d status to of miner %s expired", deal.DealID, addr)
		}

		// not check market piece status , maybe skip Packing and update to proving status directly
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, ts.Key())
		if err != nil {
			// todo if deal not found maybe need to market storage deal as error
			return fmt.Errorf("get market deal for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		if dealProposal.State.SectorStartEpoch > -1 { // include in sector
			err = dealTracker.storageRepo.UpdateDealStatus(ctx, deal.ProposalCid, storagemarket.StorageDealActive, market.Proving)
			if err != nil {
				log.Errorf("update deal status to active for sector %d of miner %s err: %s", deal.SectorNumber, addr, err)
				continue
			}

			dealTracker.eventPublisher.PublishWithCid(storagemarket.ProviderEventDealActivated, deal.ProposalCid)

			continue
		}

		if deal.State == storagemarket.StorageDealAwaitingPreCommit && deal.PieceStatus == market.Assigned {
			preInfo, err := dealTracker.fullNode.StateSectorPreCommitInfo(ctx, addr, deal.SectorNumber, ts.Key())
			if err != nil {
				if strings.Contains(err.Error(), "not found") { // todo remove this check after nv17 update
					continue
				}
				return fmt.Errorf("get precommit info for sector %d of miner %s: %w", deal.SectorNumber, addr, err)
			}

			if preInfo == nil { // PreCommit maybe not submitted
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
				return fmt.Errorf("update deal status to sealing for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}

			dealTracker.eventPublisher.PublishWithCid(storagemarket.ProviderEventDealHandedOff, deal.ProposalCid)
		}
	}
	return nil
}

func (dealTracker *DealTracker) checkSlash(ctx metrics.MetricsCtx, addr address.Address, ts *vTypes.TipSet) error {
	deals, err := dealTracker.storageRepo.GetDealByAddrAndStatus(ctx, addr, storagemarket.StorageDealActive)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return fmt.Errorf("get miner %s storage deals for check StorageDealActive %w", addr, err)
	}

	for _, deal := range deals {
		dealProposal, err := dealTracker.fullNode.StateMarketStorageDeal(ctx, deal.DealID, ts.Key())
		if err != nil {
			return fmt.Errorf("get market deal info for sector %d of miner %s %w", deal.SectorNumber, addr, err)
		}
		if dealProposal.State.SlashEpoch > -1 {
			err = dealTracker.storageRepo.UpdateDealStatus(ctx, deal.ProposalCid, storagemarket.StorageDealSlashed, "")
			if err != nil {
				return fmt.Errorf("update deal status to slash for sector %d of miner %s %w", deal.SectorNumber, addr, err)
			}

			contextID := deal.ProposalCid.Bytes()
			_, err = dealTracker.indexProviderMgr.AnnounceDealRemoved(ctx, deal.Proposal.Provider, contextID)
			if err != nil {
				return fmt.Errorf("announce deal %v removed failed: %v", deal.ProposalCid, err)
			}
		}
	}
	return nil
}
