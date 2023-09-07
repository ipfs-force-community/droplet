package badger

import (
	"context"
	"encoding/json"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
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

func (r *directDealRepo) ListDeal(ctx context.Context) (deals []*types.DirectDeal, err error) {
	result, err := r.ds.Query(ctx, query.Query{})
	if err != nil {
		return nil, err
	}
	defer func() {
		err = result.Close()
	}()

	for entry := range result.Next() {
		if entry.Error != nil {
			return nil, err
		}
		var d types.DirectDeal
		err = json.Unmarshal(entry.Value, &d)
		if err != nil {
			return nil, err
		}

		deals = append(deals, &d)
	}

	return deals, nil
}

var _ repo.DirectDealRepo = (*directDealRepo)(nil)

func keyFromID(id uuid.UUID) datastore.Key {
	return datastore.KeyWithNamespaces([]string{id.String()})
}
