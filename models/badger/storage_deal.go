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
	"github.com/filecoin-project/venus-market/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
)

type storageDealRepo struct {
	ds datastore.Batching
}

var _ (repo.StorageDealRepo) = (*storageDealRepo)(nil)

func NewStorageDealRepo(ds StorageDealsDS) *storageDealRepo {
	return &storageDealRepo{ds}
}

func (sdr *storageDealRepo) SaveDeal(ctx context.Context, storageDeal *types.MinerDeal) error {
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

func (sdr *storageDealRepo) GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*types.MinerDeal, error) {
	var startIdx, idx = pageIndex * pageSize, 0
	var storageDeals []*types.MinerDeal
	var err error
	if err = travelDeals(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
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
	if err = travelDeals(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
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

func (sdr *storageDealRepo) GetDealByAddrAndStatus(ctx context.Context, addr address.Address, statues ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	filter := map[storagemarket.StorageDealStatus]struct{}{}
	for _, status := range statues {
		filter[status] = struct{}{}
	}

	var storageDeals []*types.MinerDeal
	var err error
	if err = travelDeals(ctx, sdr.ds,
		func(deal *types.MinerDeal) (stop bool, err error) {
			if deal.ClientDealProposal.Proposal.Provider == addr {
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

func (sdr *storageDealRepo) UpdateDealStatus(ctx context.Context, proposalCid cid.Cid, status storagemarket.StorageDealStatus, pieceState string) error {
	deal, err := sdr.GetDeal(ctx, proposalCid)
	if err != nil {
		return err
	}
	var updateColumns = 0
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
	if err := travelDeals(ctx, sdr.ds, func(deal *types.MinerDeal) (stop bool, err error) {
		if deal.ClientDealProposal.Proposal.Provider == miner {
			storageDeals = append(storageDeals, deal)
		}
		return
	}); err != nil {
		return nil, err
	}

	return storageDeals, nil
}

func (sdr *storageDealRepo) ListDeal(ctx context.Context) ([]*types.MinerDeal, error) {
	storageDeals := make([]*types.MinerDeal, 0)
	if err := travelDeals(ctx, sdr.ds, func(deal *types.MinerDeal) (bool, error) {
		storageDeals = append(storageDeals, deal)
		return false, nil
	}); err != nil {
		return nil, err
	}
	return storageDeals, nil
}

func (m *storageDealRepo) GetPieceInfo(ctx context.Context, pieceCID cid.Cid) (*piecestore.PieceInfo, error) {
	var pieceInfo = piecestore.PieceInfo{
		PieceCID: pieceCID,
		Deals:    nil,
	}
	var err error
	if err = travelDeals(ctx, m.ds, func(deal *types.MinerDeal) (bool, error) {
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

func (dsr *storageDealRepo) ListPieceInfoKeys(ctx context.Context) ([]cid.Cid, error) {
	var cidsMap = make(map[cid.Cid]interface{})
	err := travelDeals(ctx, dsr.ds,
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

func (dsr *storageDealRepo) GetDealByDealID(ctx context.Context, mAddr address.Address, dealID abi.DealID) (*types.MinerDeal, error) {
	var deal *types.MinerDeal
	var err error
	if err = travelDeals(ctx, dsr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
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

func (dsr *storageDealRepo) GetDealsByPieceStatusV0(ctx context.Context, mAddr address.Address, pieceStatus string) ([]*types.MinerDeal, error) {
	var deals []*types.MinerDeal
	var err error
	if err = travelDeals(ctx, dsr.ds,
		func(inDeal *types.MinerDeal) (bool, error) {
			if inDeal.ClientDealProposal.Proposal.Provider == mAddr && inDeal.PieceStatus == pieceStatus {
				deals = append(deals, inDeal)
			}
			return false, nil
		}); err != nil {
		return nil, err
	}

	return deals, nil
}

func (dsr *storageDealRepo) GetDealsByPieceStatus(ctx context.Context, mAddr address.Address, pieceStatus string) ([]*types.MinerDeal, error) {
	var deals []*types.MinerDeal

	return deals, travelDeals(ctx, dsr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
		if inDeal.ClientDealProposal.Proposal.Provider == mAddr && inDeal.PieceStatus == pieceStatus {
			deals = append(deals, inDeal)
		}
		return false, nil
	})
}

func (sdr *storageDealRepo) GetPieceSize(ctx context.Context, pieceCID cid.Cid) (abi.UnpaddedPieceSize, abi.PaddedPieceSize, error) {
	var deal *types.MinerDeal

	err := travelDeals(ctx, sdr.ds, func(inDeal *types.MinerDeal) (stop bool, err error) {
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
