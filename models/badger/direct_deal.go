package badger

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-state-types/abi"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
)

func NewDirectDealRepo(ds DirectDealsDS) repo.DirectDealRepo {
	return &directDealRepo{ds: ds}
}

type directDealRepo struct {
	ds datastore.Batching
}

func (r *directDealRepo) SaveDeal(ctx context.Context, d *types.DirectDeal) error {
	key := keyFromID(d.ID)
	d.TimeStamp = makeRefreshedTimeStamp(&d.TimeStamp)
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return r.ds.Put(ctx, key, data)
}

func (r *directDealRepo) SaveDealWithState(ctx context.Context, deal *types.DirectDeal, state types.DirectDealState) error {
	d, err := r.GetDeal(ctx, deal.ID)
	if err != nil {
		return err
	}
	if d.State != state {
		return fmt.Errorf("expected deal state %d, but got %d", state, d.State)
	}
	return r.SaveDeal(ctx, deal)
}

func (r *directDealRepo) GetDeal(ctx context.Context, id uuid.UUID) (*types.DirectDeal, error) {
	key := keyFromID(id)
	data, err := r.ds.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var d types.DirectDeal
	err = json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func (r *directDealRepo) GetDealByAllocationID(ctx context.Context, allocationID uint64) (*types.DirectDeal, error) {
	var d *types.DirectDeal
	err := travelJSONAbleDS(ctx, r.ds, func(deal *types.DirectDeal) (bool, error) {
		if deal.AllocationID == allocationID {
			d = deal
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, repo.ErrNotFound
	}

	return d, nil
}

func (r *directDealRepo) GetDealsByMinerAndState(ctx context.Context, miner address.Address, state types.DirectDealState) ([]*types.DirectDeal, error) {
	var deals []*types.DirectDeal
	err := travelJSONAbleDS(ctx, r.ds, func(deal *types.DirectDeal) (bool, error) {
		if deal.Provider == miner && deal.State == state {
			deals = append(deals, deal)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return deals, nil
}

func (r *directDealRepo) GetPieceInfo(ctx context.Context, pieceCID cid.Cid) (*piecestore.PieceInfo, error) {
	pieceInfo := piecestore.PieceInfo{
		PieceCID: pieceCID,
		Deals:    nil,
	}
	var err error
	if err = travelJSONAbleDS(ctx, r.ds, func(deal *types.DirectDeal) (bool, error) {
		if deal.PieceCID.Equals(pieceCID) {
			pieceInfo.Deals = append(pieceInfo.Deals, piecestore.DealInfo{
				SectorID: deal.SectorID,
				Offset:   deal.Offset,
				Length:   deal.PieceSize,
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

func (r *directDealRepo) GetPieceSize(ctx context.Context, pieceCID cid.Cid) (uint64, abi.PaddedPieceSize, error) {
	var deal *types.DirectDeal
	err := travelJSONAbleDS(ctx, r.ds, func(inDeal *types.DirectDeal) (stop bool, err error) {
		if inDeal.PieceCID == pieceCID && inDeal.State != types.DealExpired {
			deal = inDeal
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return 0, 0, err
	}
	if deal == nil {
		return 0, 0, repo.ErrNotFound
	}
	return deal.PayloadSize, deal.PieceSize, nil
}

func (r *directDealRepo) ListDeal(ctx context.Context, params types.DirectDealQueryParams) ([]*types.DirectDeal, error) {
	var deals []*types.DirectDeal
	end := params.Limit + params.Offset

	var count int
	err := travelJSONAbleDS(ctx, r.ds, func(deal *types.DirectDeal) (bool, error) {
		if count >= end {
			return true, nil
		}
		if params.State != nil && deal.State != *params.State {
			return false, nil
		}
		if !params.Provider.Empty() && deal.Provider != params.Provider {
			return false, nil
		}
		if !params.Client.Empty() && deal.Client != params.Client {
			return false, nil
		}

		if count >= params.Offset && count < end {
			deals = append(deals, deal)
		}
		count++

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return deals, nil
}

var _ repo.DirectDealRepo = (*directDealRepo)(nil)

func keyFromID(id uuid.UUID) datastore.Key {
	return datastore.KeyWithNamespaces([]string{id.String()})
}
