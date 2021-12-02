package badger

import (
	"bytes"
	"fmt"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-core/peer"
)

const RetrievalDealTableName = "retrieval_deals"

type retrievalDealRepo struct {
	ds datastore.Batching
}

func NewRetrievalDealRepo(ds RetrievalProviderDS) repo.IRetrievalDealRepo {
	return &retrievalDealRepo{ds}
}

func (r retrievalDealRepo) SaveDeal(deal *types.ProviderDealState) error {
	b, err := cborrpc.Dump(deal)
	if err != nil {
		return err
	}

	fmt.Println("save deal ", deal.Identifier(), deal.Status.String())

	return r.ds.Put(statestore.ToKey(deal.Identifier()), b)
}

func (r retrievalDealRepo) GetDeal(id peer.ID, id2 retrievalmarket.DealID) (*types.ProviderDealState, error) {
	key := statestore.ToKey(retrievalmarket.ProviderDealIdentifier{
		Receiver: id,
		DealID:   id2,
	})

	value, err := r.ds.Get(key)
	if err != nil {
		return nil, err
	}
	var retrievalDeal types.ProviderDealState
	if err := cborrpc.ReadCborRPC(bytes.NewReader(value), &retrievalDeal); err != nil {
		return nil, err
	}

	fmt.Println("get deal ", key.String(), retrievalDeal.Status.String())
	return &retrievalDeal, nil
}

func (r retrievalDealRepo) GetDealByTransferId(chid datatransfer.ChannelID) (*types.ProviderDealState, error) {
	var result *types.ProviderDealState
	err := travelDeals(r.ds, func(deal *types.ProviderDealState) (stop bool, err error) {
		if deal.ChannelID != nil && deal.ChannelID.Initiator == chid.Initiator && deal.ChannelID.Responder == chid.Responder && deal.ChannelID.ID == chid.ID {
			result = deal
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, repo.ErrNotFound
	}
	return result, nil
}

func (r retrievalDealRepo) HasDeal(id peer.ID, id2 retrievalmarket.DealID) (bool, error) {
	return r.ds.Has(statestore.ToKey(retrievalmarket.ProviderDealIdentifier{
		Receiver: id,
		DealID:   id2,
	}))
}

func (r retrievalDealRepo) ListDeals(pageIndex, pageSize int) ([]*types.ProviderDealState, error) {
	result, err := r.ds.Query(query.Query{})
	if err != nil {
		return nil, err
	}

	defer result.Close() //nolint:errcheck

	retrievalDeals := make([]*types.ProviderDealState, 0)
	for res := range result.Next() {
		if res.Error != nil {
			return nil, err
		}
		var deal types.ProviderDealState
		if err := cborrpc.ReadCborRPC(bytes.NewReader(res.Value), &deal); err != nil {
			return nil, err
		}
		retrievalDeals = append(retrievalDeals, &deal)
	}

	return retrievalDeals, nil
}
