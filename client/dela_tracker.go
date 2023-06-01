package client

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
)

const (
	maxEOFCount = 3
)

var dealTrackerLog = logging.Logger("deal-tracker")

type DealTracker struct {
	full     v1api.FullNode
	dealRepo repo.ClientOfflineDealRepo
	stream   *ClientStream
	// todo: loop update miner info?
	minerInfo map[address.Address]shared.MinerInfo
	eofErrs   map[cid.Cid]int
}

func NewDealTracker(lc fx.Lifecycle,
	full v1api.FullNode,
	offlineDealRepo repo.ClientOfflineDealRepo,
	stream *ClientStream,
) *DealTracker {
	dt := &DealTracker{
		full:      full,
		dealRepo:  offlineDealRepo,
		stream:    stream,
		minerInfo: make(map[address.Address]shared.MinerInfo),
		eofErrs:   make(map[cid.Cid]int),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go dt.loopRefreshDealState(ctx)

			return nil
		},
	})

	return dt
}

type dealInfos struct {
	activeDeals, inactiveDeals []*types.ClientOfflineDeal
	miners                     map[address.Address]struct{}
}

func (dt *DealTracker) loadDeals(ctx context.Context) (*dealInfos, error) {
	deals, err := dt.dealRepo.ListDeal(ctx)
	if err != nil {
		return nil, err
	}

	infos := &dealInfos{miners: make(map[address.Address]struct{})}
	for _, deal := range deals {
		if storageprovider.IsTerminateState(deal.State) {
			continue
		}
		if deal.State != storagemarket.StorageDealActive {
			infos.inactiveDeals = append(infos.inactiveDeals, deal)
		} else {
			infos.activeDeals = append(infos.activeDeals, deal)
		}
		infos.miners[deal.Proposal.Provider] = struct{}{}
	}

	return infos, nil
}

func (dt *DealTracker) loopRefreshDealState(ctx context.Context) {
	infos, err := dt.loadDeals(ctx)
	if err == nil {
		dt.checkExpired(ctx, infos.inactiveDeals)
		dt.refreshDealState(ctx, infos)
		dt.checkSlash(ctx, infos.activeDeals)
	}

	ticker := time.NewTicker(time.Minute * 3)
	defer ticker.Stop()

	slashTicker := time.NewTimer(time.Hour * 6)
	defer slashTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			infos, err := dt.loadDeals(ctx)
			if err != nil {
				dealTrackerLog.Infof("list offline deal failed: %v", err)
				continue
			}
			dt.checkExpired(ctx, infos.inactiveDeals)
			dt.refreshDealState(ctx, infos)
		case <-slashTicker.C:
			infos, err := dt.loadDeals(ctx)
			if err != nil {
				dealTrackerLog.Infof("list offline deal failed: %v", err)
				continue
			}
			dt.checkSlash(ctx, infos.activeDeals)
		}
	}
}

func (dt *DealTracker) updateMinerCache(ctx context.Context, miners map[address.Address]struct{}) error {
	for miner := range miners {
		_, ok := dt.minerInfo[miner]
		if !ok {
			minerInfo, err := dt.full.StateMinerInfo(ctx, miner, shared.EmptyTSK)
			if err != nil {
				return fmt.Errorf("got miner info failed: %v", err)
			}
			dt.minerInfo[miner] = minerInfo
		}
	}

	return nil
}

func (dt *DealTracker) refreshDealState(ctx context.Context, infos *dealInfos) {
	if err := dt.updateMinerCache(ctx, infos.miners); err != nil {
		dealTrackerLog.Info(err.Error())
	}

	for _, deal := range infos.inactiveDeals {
		proposalCID := deal.ProposalCID
		minerInfo, ok := dt.minerInfo[deal.Proposal.Provider]
		if !ok || minerInfo.PeerId == nil {
			dealTrackerLog.Debugf("deal %s not found miner peer", proposalCID)
			continue
		}
		if dt.eofErrs[proposalCID] >= maxEOFCount {
			continue
		}
		status, err := dt.stream.GetDealState(ctx, deal, minerInfo)
		if err != nil {
			if strings.Contains(err.Error(), io.EOF.Error()) {
				dt.eofErrs[proposalCID]++
			}
			dealTrackerLog.Infof("failed to got deal status: %v %v", proposalCID, err)
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

func (dt *DealTracker) checkExpired(ctx context.Context, deals []*types.ClientOfflineDeal) {
	head, err := dt.full.ChainHead(ctx)
	if err != nil {
		dealTrackerLog.Infof("got chain head failed: %v", err)
		return
	}
	for _, deal := range deals {
		if deal.Proposal.StartEpoch < head.Height() {
			deal.State = storagemarket.StorageDealExpired
			dt.persistDeal(ctx, deal)
		}
	}
}

func (dt *DealTracker) checkSlash(ctx context.Context, deals []*types.ClientOfflineDeal) {
	for _, deal := range deals {
		md, err := dt.full.StateMarketStorageDeal(ctx, abi.DealID(deal.DealID), shared.EmptyTSK)
		if err == nil && md.State.SlashEpoch > -1 {
			deal.State = storagemarket.StorageDealSlashed
			dt.persistDeal(ctx, deal)
		}
	}
}

func (dt *DealTracker) persistDeal(ctx context.Context, deal *types.ClientOfflineDeal) {
	deal.UpdatedAt = time.Now()
	if err := dt.dealRepo.SaveDeal(ctx, deal); err != nil {
		dealTrackerLog.Errorf("failed to save deal: %s %v", deal.ProposalCID, err)
	}
}
