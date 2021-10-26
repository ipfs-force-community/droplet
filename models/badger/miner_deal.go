package badger

import (
	"bytes"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/itf"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

type minerDealStore struct {
	ds datastore.Batching
}

func NewMinerDealStore(ds itf.ProviderDealDS) *minerDealStore {
	return &minerDealStore{ds}
}

func (m *minerDealStore) SaveMinerDeal(minerDeal *storagemarket.MinerDeal) error {
	b, err := cborrpc.Dump(minerDeal)
	if err != nil {
		return err
	}

	return m.ds.Put(statestore.ToKey(minerDeal.ProposalCid), b)
}

func (m *minerDealStore) GetMinerDeal(proposalCid cid.Cid) (*storagemarket.MinerDeal, error) {
	value, err := m.ds.Get(statestore.ToKey(proposalCid))
	if err != nil {
		return nil, err
	}
	var minerDeal storagemarket.MinerDeal
	if err := minerDeal.UnmarshalCBOR(bytes.NewReader(value)); err != nil {
		return nil, err
	}

	return &minerDeal, nil
}

func (m *minerDealStore) ListMinerDeal() ([]*storagemarket.MinerDeal, error) {
	result, err := m.ds.Query(query.Query{})
	if err != nil {
		return nil, err
	}
	defer result.Close() //nolint:errcheck

	minerDeals := make([]*storagemarket.MinerDeal, 0)
	for res := range result.Next() {
		if res.Error != nil {
			return nil, err
		}
		var deal storagemarket.MinerDeal
		if err := deal.UnmarshalCBOR(bytes.NewReader(res.Value)); err != nil {
			return nil, err
		}
		minerDeals = append(minerDeals, &deal)
	}

	return minerDeals, nil
}
