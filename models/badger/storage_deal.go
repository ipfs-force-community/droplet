package badger

import (
	"bytes"

	"github.com/filecoin-project/go-address"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

type storageDealRepo struct {
	ds datastore.Batching
}

func NewStorageDealRepo(ds repo.ProviderDealDS) *storageDealRepo {
	return &storageDealRepo{ds}
}

func (sdr *storageDealRepo) SaveDeal(storageDeal *storagemarket.MinerDeal) error {
	b, err := cborrpc.Dump(storageDeal)
	if err != nil {
		return err
	}

	return sdr.ds.Put(statestore.ToKey(storageDeal.ProposalCid), b)
}

func (sdr *storageDealRepo) GetDeal(proposalCid cid.Cid) (*storagemarket.MinerDeal, error) {
	value, err := sdr.ds.Get(statestore.ToKey(proposalCid))
	if err != nil {
		return nil, err
	}
	var StorageDeal storagemarket.MinerDeal
	if err := StorageDeal.UnmarshalCBOR(bytes.NewReader(value)); err != nil {
		return nil, err
	}

	return &StorageDeal, nil
}

func (sdr *storageDealRepo) ListDeal(mAddr address.Address) ([]storagemarket.MinerDeal, error) {
	result, err := sdr.ds.Query(query.Query{})
	if err != nil {
		return nil, err
	}
	defer result.Close() //nolint:errcheck

	storageDeals := make([]storagemarket.MinerDeal, 0)
	for res := range result.Next() {
		if res.Error != nil {
			return nil, err
		}
		var deal storagemarket.MinerDeal
		if err := deal.UnmarshalCBOR(bytes.NewReader(res.Value)); err != nil {
			return nil, err
		}
		if deal.Proposal.Provider == mAddr {
			storageDeals = append(storageDeals, deal)
		}
	}

	return storageDeals, nil
}
