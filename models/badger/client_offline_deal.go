package badger

import (
	"context"
	"encoding/json"

	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

func NewBadgerClientOfflineDealRepo(ds ClientOfflineDealsDS) repo.ClientOfflineDealRepo {
	return &badgerClientOfflineDealRepo{ds: ds}
}

type badgerClientOfflineDealRepo struct {
	ds datastore.Batching
}

func (r *badgerClientOfflineDealRepo) SaveDeal(ctx context.Context, d *types.ClientOfflineDeal) error {
	key := keyFromProposalCID(d.ProposalCID)
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return r.ds.Put(ctx, key, data)
}

func (r *badgerClientOfflineDealRepo) GetDeal(ctx context.Context, proposalCID cid.Cid) (*types.ClientOfflineDeal, error) {
	key := keyFromProposalCID(proposalCID)
	data, err := r.ds.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var d types.ClientOfflineDeal
	err = json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func (r *badgerClientOfflineDealRepo) ListDeal(ctx context.Context) (deals []*types.ClientOfflineDeal, err error) {
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
		var d types.ClientOfflineDeal
		err = json.Unmarshal(entry.Value, &d)
		if err != nil {
			return nil, err
		}

		deals = append(deals, &d)
	}

	return deals, nil
}

var _ repo.ClientOfflineDealRepo = (*badgerClientOfflineDealRepo)(nil)

func keyFromProposalCID(proposalCID cid.Cid) datastore.Key {
	return datastore.KeyWithNamespaces([]string{proposalCID.String()})
}
