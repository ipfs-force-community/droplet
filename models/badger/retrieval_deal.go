package badger

import (
	"bytes"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-core/peer"
)

const RetrievalDealTableName = "retrieval_deals"

type retrievalDealRepo struct {
	ds datastore.Batching
}

func NewRetrievalDealRepo(ds repo.RetrievalProviderDS) repo.IRetrievalDealRepo {
	return &retrievalDealRepo{ds}
}

func (r retrievalDealRepo) SaveDeal(deal *retrievalmarket.ProviderDealState) error {
	b, err := cborrpc.Dump(deal)
	if err != nil {
		return err
	}

	return r.ds.Put(statestore.ToKey(deal.Identifier()), b)
}

func (r retrievalDealRepo) GetDeal(id peer.ID, id2 retrievalmarket.DealID) (*retrievalmarket.ProviderDealState, error) {
	value, err := r.ds.Get(statestore.ToKey(retrievalmarket.ProviderDealIdentifier{
		Receiver: id,
		DealID:   id2,
	}))
	if err != nil {
		return nil, err
	}
	var retrievalDeal retrievalmarket.ProviderDealState
	if err := retrievalDeal.UnmarshalCBOR(bytes.NewReader(value)); err != nil {
		return nil, err
	}

	return &retrievalDeal, nil
}

func (r retrievalDealRepo) HasDeal(id peer.ID, id2 retrievalmarket.DealID) (bool, error) {
	return r.ds.Has(statestore.ToKey(retrievalmarket.ProviderDealIdentifier{
		Receiver: id,
		DealID:   id2,
	}))
}

func (r retrievalDealRepo) ListDeals(pageIndex, pageSize int) ([]*retrievalmarket.ProviderDealState, error) {
	result, err := r.ds.Query(query.Query{})
	if err != nil {
		return nil, err
	}

	defer result.Close() //nolint:errcheck

	retrievalDeals := make([]*retrievalmarket.ProviderDealState, 0)
	for res := range result.Next() {
		if res.Error != nil {
			return nil, err
		}
		var deal retrievalmarket.ProviderDealState
		if err := deal.UnmarshalCBOR(bytes.NewReader(res.Value)); err != nil {
			return nil, err
		}
		retrievalDeals = append(retrievalDeals, &deal)
	}

	return retrievalDeals, nil
}
