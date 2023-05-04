package client

import (
	"context"
	"sync"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-auth/log"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	"github.com/filecoin-project/venus-market/v2/types"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"go.uber.org/fx"
)

type OfflineDealManager struct {
	full       v1api.FullNode
	dealRepo   repo.ClientOfflineDealRepo
	dealClient storagemarket.StorageClient

	deals         map[cid.Cid]*types.ClientOfflineDeal
	inactiveDeals map[cid.Cid]struct{}

	lk sync.RWMutex
}

func newOfflineDealManager(lc fx.Lifecycle,
	full v1api.FullNode,
	offlineDealRepo repo.ClientOfflineDealRepo,
	dealClient storagemarket.StorageClient,
) *OfflineDealManager {
	mgr := &OfflineDealManager{
		full:     full,
		dealRepo: offlineDealRepo,

		deals:         make(map[cid.Cid]*types.ClientOfflineDeal),
		inactiveDeals: make(map[cid.Cid]struct{}),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := mgr.loadDeals(ctx); err != nil {
				return err
			}
			go mgr.loopRefreshDealState(ctx)

			return nil
		},
	})

	return mgr
}

func (mgr *OfflineDealManager) loadDeals(ctx context.Context) error {
	deals, err := mgr.dealRepo.ListDeal(ctx)
	if err != nil {
		return err
	}

	for _, deal := range deals {
		if storageprovider.IsTerminateState(deal.State) {
			continue
		}
		if deal.State != storagemarket.StorageDealActive {
			mgr.inactiveDeals[deal.ProposalCID] = struct{}{}
		}

		mgr.deals[deal.ProposalCID] = deal
	}

	return nil
}

func (mgr *OfflineDealManager) loopRefreshDealState(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 30)
	defer ticker.Stop()

	slashTicker := time.NewTimer(time.Hour * 3)
	defer slashTicker.Stop()

	mgr.refreshDealState(ctx)
	mgr.checkSlash(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mgr.refreshDealState(ctx)
		case <-slashTicker.C:
			mgr.checkSlash(ctx)
		}
	}
}

func (mgr *OfflineDealManager) refreshDealState(ctx context.Context) {
	var activeDeals []cid.Cid
	var terminateDeals []cid.Cid

	for proposalCID := range mgr.inactiveDeals {
		deal, ok := mgr.getDeal(proposalCID)
		if !ok {
			continue
		}

		status, err := mgr.dealClient.GetProviderDealState(ctx, proposalCID)
		if err != nil {
			log.Warnf("failed to got deal status: %v %v", proposalCID, err)
			continue
		}
		if deal.State != status.State || deal.DealID != uint64(status.DealID) {
			deal.State = status.State
			deal.DealID = uint64(status.DealID)

			mgr.persistDeal(ctx, deal)
			mgr.updateDeal(deal)

			if storageprovider.IsTerminateState(status.State) {
				terminateDeals = append(terminateDeals, proposalCID)
			}
			if status.State == storagemarket.StorageDealActive {
				activeDeals = append(activeDeals, proposalCID)
			}
		}
	}

	for _, c := range activeDeals {
		delete(mgr.inactiveDeals, c)
	}
	for _, c := range terminateDeals {
		delete(mgr.inactiveDeals, c)
		mgr.removeDeal(c)
	}

	log.Debugf("remain pending deals: %d", len(mgr.inactiveDeals))
}

func (mgr *OfflineDealManager) checkSlash(ctx context.Context) {
	activeDeals := mgr.activeDeals()
	for i, deal := range activeDeals {
		md, err := mgr.full.StateMarketStorageDeal(ctx, abi.DealID(deal.DealID), shared.EmptyTSK)
		if err == nil && md.State.SlashEpoch > -1 {
			activeDeals[i].State = storagemarket.StorageDealSlashed

			mgr.persistDeal(ctx, &activeDeals[i])
			mgr.updateDeal(&activeDeals[i])
			mgr.removeDeal(deal.ProposalCID)
		}
	}
}

func (mgr *OfflineDealManager) persistDeal(ctx context.Context, deal *types.ClientOfflineDeal) {
	if err := mgr.dealRepo.SaveDeal(ctx, deal); err != nil {
		log.Errorf("failed to save deal: %s %v", deal.ProposalCID, err)
	}
}

func (mgr *OfflineDealManager) getDeal(proposalCID cid.Cid) (*types.ClientOfflineDeal, bool) {
	mgr.lk.RLock()
	defer mgr.lk.RUnlock()

	deal, ok := mgr.deals[proposalCID]
	if ok {
		dealCopy := types.ClientOfflineDeal{}
		dealCopy = *deal

		return &dealCopy, ok
	}

	return nil, false
}

func (mgr *OfflineDealManager) addDeal(deal *types.ClientOfflineDeal) {
	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	mgr.deals[deal.ProposalCID] = deal
	mgr.inactiveDeals[deal.ProposalCID] = struct{}{}
}

func (mgr *OfflineDealManager) removeDeal(proposalCID cid.Cid) {
	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	delete(mgr.deals, proposalCID)
	delete(mgr.inactiveDeals, proposalCID)
}

func (mgr *OfflineDealManager) updateDeal(deal *types.ClientOfflineDeal) {
	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	_, ok := mgr.deals[deal.ProposalCID]
	if ok {
		mgr.deals[deal.ProposalCID] = deal
	}
}

func (mgr *OfflineDealManager) activeDeals() []types.ClientOfflineDeal {
	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	deals := make([]types.ClientOfflineDeal, 0, len(mgr.deals))
	for _, deal := range mgr.deals {
		if deal.State == storagemarket.StorageDealActive {
			deals = append(deals, *deal)
		}
	}

	return deals
}

func (mgr *OfflineDealManager) VerifiedDealProposals() []*shared.ClientDealProposal {
	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	deals := make([]*shared.ClientDealProposal, 0, len(mgr.deals))
	for i, deal := range mgr.deals {
		if !storageprovider.IsTerminateState(deal.State) && deal.Proposal.VerifiedDeal {
			deals = append(deals, &mgr.deals[i].ClientDealProposal)
		}
	}

	return deals
}
