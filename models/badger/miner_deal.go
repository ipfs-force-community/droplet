package badger

import (
	"bytes"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

type minerDealStore struct {
	ds datastore.Batching
}

func NewMinerDealStore(ds models.ProviderDealDS) *minerDealStore {
	return &minerDealStore{ds}
}

func (m *minerDealStore) SaveMinerDeal(minerDeal *types.MinerDeal) error {
	b, err := cborrpc.Dump(minerDeal)
	if err != nil {
		return err
	}

	return m.ds.Put(statestore.ToKey(minerDeal.ProposalCid), b)
}

func (m *minerDealStore) GetMinerDeal(proposalCid cid.Cid) (*types.MinerDeal, error) {
	value, err := m.ds.Get(statestore.ToKey(proposalCid))
	if err != nil {
		return nil, err
	}
	var minerDeal types.MinerDeal
	if err := minerDeal.UnmarshalCBOR(bytes.NewReader(value)); err != nil {
		return nil, err
	}

	return &minerDeal, nil
}

func (m *minerDealStore) UpdateMinerDeal(proposalCid cid.Cid, updateCols map[string]interface{}) error {
	panic("implement me")
}

func (m *minerDealStore) ListMinerDeal() ([]*types.MinerDeal, error) {
	result, err := m.ds.Query(query.Query{})
	if err != nil {
		return nil, err
	}
	defer result.Close() //nolint:errcheck

	minerDeals := make([]*types.MinerDeal, 0)
	for res := range result.Next() {
		if res.Error != nil {
			return nil, err
		}
		var deal types.MinerDeal
		if deal.UnmarshalCBOR(bytes.NewReader(res.Value)); err != nil {
			return nil, err
		}
		minerDeals = append(minerDeals, &deal)
	}

	return minerDeals, nil
}

//var _ repo.MinerDealRepo = (*minerDealStore)(nil)
