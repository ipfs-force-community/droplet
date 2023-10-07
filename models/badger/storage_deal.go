package badger

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-statestore"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
)

type storageDealRepo struct {
	ds datastore.Batching
}

var _ (repo.StorageDealRepo) = (*storageDealRepo)(nil)

func NewStorageDealRepo(ds StorageDealsDS) repo.StorageDealRepo {
	return &storageDealRepo{ds}
}

func (sdr *storageDealRepo) CreateDeals(ctx context.Context, storageDeals []*types.MinerDeal) error {
	for _, storageDeal := range storageDeals {
		if err := sdr.SaveDeal(ctx, storageDeal); err != nil {
			return err
		}
	}

	return nil
}

func (sdr *storageDealRepo) SaveDeal(ctx context.Context, storageDeal *types.MinerDeal) error {
	storageDeal.TimeStamp = makeRefreshedTimeStamp(&storageDeal.TimeStamp)
	b, err := cborrpc.Dump(storageDeal)
	if err != nil {
		return err
	}
	return sdr.ds.Put(ctx, statestore.ToKey(storageDeal.ProposalCid), b)
}

func (sdr *storageDealRepo) GetDeal(ctx context.Context, proposalCid cid.Cid) (*types.MinerDeal, error) {
	value, err := sdr.ds.Get(ctx, statestore.ToKey(proposalCid))
	if err != nil {
		return nil, err
	}
	var deal types.MinerDeal
	if err = cborrpc.ReadCborRPC(bytes.NewReader(value), &deal); err != nil {
		return nil, err
	}

	return &deal, nil
}

func (sdr *storageDealRepo) GetDealByUUID(ctx context.Context, id uuid.UUID) (*types.MinerDeal, error) {
	var out *types.MinerDeal
	if err := travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if deal.ID == id {
			out = deal
			return true, nil
		}
		return
	}); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, repo.ErrNotFound
	}

	return out, nil
}

func (sdr *storageDealRepo) GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*types.MinerDeal, error) {
	startIdx, idx := pageIndex*pageSize, 0
	var storageDeals []*types.MinerDeal
	var err error
	if err = travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if deal.ClientDealProposal.Proposal.Provider != miner {
			return
		}
		idx++
		if idx-1 < startIdx {
			return
		}
		storageDeals = append(storageDeals, deal)
		if len(storageDeals) >= pageSize {
			return true, nil
		}
		return
	}); err != nil {
		return nil, err
	}

	if len(storageDeals) == 0 {
		err = repo.ErrNotFound
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) GetDealsByPieceCidAndStatus(ctx context.Context, piececid cid.Cid, statues ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	filter := map[storagemarket.StorageDealStatus]struct{}{}
	for _, status := range statues {
		filter[status] = struct{}{}
	}

	var storageDeals []*types.MinerDeal
	var err error
	if err = travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if deal.ClientDealProposal.Proposal.PieceCID != piececid {
			return
		}

		if _, ok := filter[deal.State]; !ok {
			return
		}

		storageDeals = append(storageDeals, deal)
		return
	}); err != nil {
		return nil, err
	}

	if len(storageDeals) == 0 {
		err = repo.ErrNotFound
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) GetDealsByDataCidAndDealStatus(ctx context.Context, mAddr address.Address, dataCid cid.Cid, pieceStatuss []types.PieceStatus) ([]*types.MinerDeal, error) {
	filter := map[types.PieceStatus]struct{}{}
	for _, status := range pieceStatuss {
		filter[status] = struct{}{}
	}

	var storageDeals []*types.MinerDeal
	var err error
	if err = travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if mAddr != address.Undef && deal.ClientDealProposal.Proposal.Provider != mAddr {
			return
		}
		if deal.Ref.Root != dataCid {
			return
		}

		if _, ok := filter[deal.PieceStatus]; !ok {
			return
		}

		storageDeals = append(storageDeals, deal)
		return
	}); err != nil {
		return nil, err
	}

	if len(storageDeals) == 0 {
		err = repo.ErrNotFound
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) GetDealByAddrAndStatus(ctx context.Context, addr address.Address, statues ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	filter := map[storagemarket.StorageDealStatus]struct{}{}
	for _, status := range statues {
		filter[status] = struct{}{}
	}

	var storageDeals []*types.MinerDeal
	var err error
	if err = travelCborAbleDS(ctx, sdr.ds,
		func(deal *types.MinerDeal) (stop bool, err error) {
			if addr == address.Undef || deal.ClientDealProposal.Proposal.Provider == addr {
				if _, ok := filter[deal.State]; !ok {
					return
				}
				storageDeals = append(storageDeals, deal)
			}
			return
		}); err != nil {
		return nil, err
	}

	if len(storageDeals) == 0 {
		err = repo.ErrNotFound
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) UpdateDealStatus(ctx context.Context, proposalCid cid.Cid, status storagemarket.StorageDealStatus, pieceState types.PieceStatus) error {
	deal, err := sdr.GetDeal(ctx, proposalCid)
	if err != nil {
		return err
	}
	updateColumns := 0
	if status != storagemarket.StorageDealUnknown {
		deal.State = status
		updateColumns++
	}
	if len(pieceState) != 0 {
		deal.PieceStatus = pieceState
		updateColumns++
	}
	if updateColumns == 0 {
		return nil
	}
	return sdr.SaveDeal(ctx, deal)
}

func (sdr *storageDealRepo) ListDealByAddr(ctx context.Context, miner address.Address) ([]*types.MinerDeal, error) {
	storageDeals := make([]*types.MinerDeal, 0)
	if err := travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if deal.ClientDealProposal.Proposal.Provider == miner {
			storageDeals = append(storageDeals, deal)
		}
		return
	}); err != nil {
		return nil, err
	}

	return storageDeals, nil
}

func (sdr *storageDealRepo) ListDeal(ctx context.Context, params *types.StorageDealQueryParams) ([]*types.MinerDeal, error) {
	var count int
	var storageDeals []*types.MinerDeal
	end := params.Limit + params.Offset

	discardFailedDeal := params.DiscardFailedDeal
	if discardFailedDeal && params.State != nil {
		state := *params.State
		discardFailedDeal = state != storagemarket.StorageDealFailing && state != storagemarket.StorageDealSlashed &&
			state != storagemarket.StorageDealExpired && state != storagemarket.StorageDealError
	}

	if err := travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if count >= end {
			return true, nil
		}

		if !params.Miner.Empty() && deal.ClientDealProposal.Proposal.Provider != params.Miner {
			return false, nil
		}
		if len(params.Client) != 0 && deal.Client.Pretty() != params.Client {
			return false, nil
		}
		if params.State != nil && deal.State != *params.State {
			return false, nil
		}
		if discardFailedDeal && (deal.State == storagemarket.StorageDealFailing || deal.State == storagemarket.StorageDealSlashed ||
			deal.State == storagemarket.StorageDealExpired || deal.State == storagemarket.StorageDealError) {
			return false, nil
		}
		if count >= params.Offset && count < end {
			storageDeals = append(storageDeals, deal)
		}
		count++

		return
	}); err != nil {
		return nil, err
	}

	return storageDeals, nil
}

func (sdr *storageDealRepo) GetPieceInfo(ctx context.Context, pieceCID cid.Cid) (*piecestore.PieceInfo, error) {
	pieceInfo := piecestore.PieceInfo{
		PieceCID: pieceCID,
		Deals:    nil,
	}
	var err error
	if err = travelCborAbleDS(ctx, sdr.ds, func(deal *types.MinerDeal) (bool, error) {
		if deal.ClientDealProposal.Proposal.PieceCID.Equals(pieceCID) {
			pieceInfo.Deals = append(pieceInfo.Deals, piecestore.DealInfo{
				DealID:   deal.DealID,
				SectorID: deal.SectorNumber,
				Offset:   deal.Offset,
				Length:   deal.Proposal.PieceSize,
			})
		}
		return false, nil
	}); err != nil {
		return nil, err
	}

	if len(pieceInfo.Deals) == 0 {
		err = repo.ErrNotFound
	}

	return &pieceInfo, err
}

func (sdr *storageDealRepo) ListPieceInfoKeys(ctx context.Context) ([]cid.Cid, error) {
	cidsMap := make(map[cid.Cid]interface{})
	err := travelCborAbleDS(ctx, sdr.ds,
		func(deal *types.MinerDeal) (bool, error) {
			cidsMap[deal.ClientDealProposal.Proposal.PieceCID] = nil
			return false, nil
		})
	if err != nil {
		return nil, err
	}

	cids := make([]cid.Cid, len(cidsMap))
	idx := 0
	for cid := range cidsMap {
		cids[idx] = cid
		idx++
	}
	return cids, nil
}

func (sdr *storageDealRepo) GetDealByDealID(ctx context.Context, mAddr address.Address, dealID abi.DealID) (*types.MinerDeal, error) {
	var deal *types.MinerDeal
	var err error
	if err = travelCborAbleDS(ctx, sdr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
		if stop = inDeal.ClientDealProposal.Proposal.Provider == mAddr && inDeal.DealID == dealID; stop {
			deal = inDeal
		}
		return stop, nil
	}); err != nil {
		return nil, err
	}
	if deal == nil {
		err = repo.ErrNotFound
	}
	return deal, err
}

func (sdr *storageDealRepo) GetDealsByPieceStatus(ctx context.Context, mAddr address.Address, pieceStatus types.PieceStatus) ([]*types.MinerDeal, error) {
	var deals []*types.MinerDeal

	return deals, travelCborAbleDS(ctx, sdr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
		if inDeal.PieceStatus != pieceStatus {
			return
		}
		if mAddr != address.Undef && inDeal.ClientDealProposal.Proposal.Provider != mAddr {
			return
		}

		deals = append(deals, inDeal)
		return false, nil
	})
}

func (sdr *storageDealRepo) GetDealsByPieceStatusAndDealStatus(ctx context.Context, mAddr address.Address, pieceStatus types.PieceStatus, dealStatus ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	var deals []*types.MinerDeal
	dict := map[storagemarket.StorageDealStatus]struct{}{}
	for _, status := range dealStatus {
		dict[status] = struct{}{}
	}

	return deals, travelCborAbleDS(ctx, sdr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
		if inDeal.PieceStatus != pieceStatus {
			return
		}
		if _, ok := dict[inDeal.State]; !ok && len(dealStatus) != 0 {
			return
		}
		if mAddr != address.Undef && inDeal.ClientDealProposal.Proposal.Provider != mAddr {
			return
		}

		deals = append(deals, inDeal)
		return false, nil
	})
}

func (sdr *storageDealRepo) GetPieceSize(ctx context.Context, pieceCID cid.Cid) (uint64, abi.PaddedPieceSize, error) {
	var deal *types.MinerDeal

	err := travelCborAbleDS(ctx, sdr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
		if inDeal.ClientDealProposal.Proposal.PieceCID == pieceCID {
			deal = inDeal
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return 0, 0, nil
	}
	if deal == nil {
		return 0, 0, repo.ErrNotFound
	}
	return deal.PayloadSize, deal.ClientDealProposal.Proposal.PieceSize, nil
}

func (sdr *storageDealRepo) GroupStorageDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[storagemarket.StorageDealStatus]int64, error) {
	result := map[storagemarket.StorageDealStatus]int64{}
	return result, travelCborAbleDS(ctx, sdr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
		if mAddr != address.Undef && mAddr != inDeal.Proposal.Provider {
			return false, nil
		}
		result[inDeal.State]++
		return false, nil
	})
}
