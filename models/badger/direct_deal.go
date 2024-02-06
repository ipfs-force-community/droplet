package badger

import (
	"context"
	"encoding/json"

	"github.com/filecoin-project/go-address"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
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
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return r.ds.Put(ctx, key, data)
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
		if deal.Provider != params.Provider {
			return false, nil
		}
		if deal.Client != params.Client {
			return false, nil
		}
		if !params.Asc {
			deals = append(deals, deal)
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
