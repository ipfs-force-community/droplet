package badger

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
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

func (r retrievalDealRepo) SaveDeal(ctx context.Context, deal *types.ProviderDealState) error {
	deal.TimeStamp = makeRefreshedTimeStamp(&deal.TimeStamp)
	b, err := cborrpc.Dump(deal)
	if err != nil {
		return err
	}
	return r.ds.Put(ctx, statestore.ToKey(deal.Identifier()), b)
}

func (r retrievalDealRepo) GetDeal(ctx context.Context, id peer.ID, id2 retrievalmarket.DealID) (*types.ProviderDealState, error) {
	key := statestore.ToKey(retrievalmarket.ProviderDealIdentifier{
		Receiver: id,
		DealID:   id2,
	})

	value, err := r.ds.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var retrievalDeal types.ProviderDealState
	if err := cborrpc.ReadCborRPC(bytes.NewReader(value), &retrievalDeal); err != nil {
		return nil, err
	}

	return &retrievalDeal, nil
}

func (r retrievalDealRepo) GetDealByTransferId(ctx context.Context, chid datatransfer.ChannelID) (*types.ProviderDealState, error) {
	var result *types.ProviderDealState
	err := travelDeals(ctx, r.ds, func(deal *types.ProviderDealState) (stop bool, err error) {
		if deal.ChannelID != nil && deal.ChannelID.Initiator == chid.Initiator && deal.ChannelID.Responder == chid.Responder && deal.ChannelID.ID == chid.ID {
			result = deal
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, repo.ErrNotFound
	}
	return result, nil
}

func (r retrievalDealRepo) HasDeal(ctx context.Context, id peer.ID, id2 retrievalmarket.DealID) (bool, error) {
	return r.ds.Has(ctx, statestore.ToKey(retrievalmarket.ProviderDealIdentifier{
		Receiver: id,
		DealID:   id2,
	}))
}

func (r retrievalDealRepo) ListDeals(ctx context.Context, pageIndex, pageSize int) ([]*types.ProviderDealState, error) {
	result, err := r.ds.Query(ctx, query.Query{})
	if err != nil {
		return nil, err
	}
	from := (pageIndex - 1) * pageSize
	to := from + pageSize
	defer result.Close() //nolint:errcheck

	var retrievalDeals []*types.ProviderDealState
	index := 0
	for res := range result.Next() {
		if index < from {
			index++
			continue
		}

		if index >= to {
			break
		}
		index++
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

func (r retrievalDealRepo) GroupRetrievalDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[retrievalmarket.DealStatus]int64, error) {
	result := map[retrievalmarket.DealStatus]int64{}
	return result, travelDeals(ctx, r.ds, func(deal *types.ProviderDealState) (stop bool, err error) {
		result[deal.Status]++
		return false, nil
	})
}
