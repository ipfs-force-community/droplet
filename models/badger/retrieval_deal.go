package badger

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
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
	err := travelCborAbleDS(ctx, r.ds, func(deal *types.ProviderDealState) (stop bool, err error) {
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

func (r retrievalDealRepo) ListDeals(ctx context.Context, params *types.RetrievalDealQueryParams) ([]*types.ProviderDealState, error) {
	var count int
	var retrievalDeals []*types.ProviderDealState
	end := params.Offset + params.Limit

	discardFailedDeal := params.DiscardFailedDeal
	if discardFailedDeal && params.Status != nil {
		discardFailedDeal = *params.Status != uint64(retrievalmarket.DealStatusErrored)
	}

	err := travelCborAbleDS(ctx, r.ds, func(deal *types.ProviderDealState) (stop bool, err error) {
		if count >= end {
			return true, nil
		}

		if len(params.Receiver) > 0 && deal.Receiver.Pretty() != params.Receiver {
			return false, nil
		}
		if len(params.PayloadCID) > 0 && deal.PayloadCID.String() != params.PayloadCID {
			return false, nil
		}
		if params.Status != nil && deal.Status != retrievalmarket.DealStatus(*params.Status) {
			return false, nil
		}
		if discardFailedDeal && deal.Status == retrievalmarket.DealStatusErrored {
			return false, nil
		}
		if count >= params.Offset && count < end {
			retrievalDeals = append(retrievalDeals, deal)
		}
		count++

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return retrievalDeals, nil
}

func (r retrievalDealRepo) GroupRetrievalDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[retrievalmarket.DealStatus]int64, error) {
	result := map[retrievalmarket.DealStatus]int64{}
	return result, travelCborAbleDS(ctx, r.ds, func(deal *types.ProviderDealState) (stop bool, err error) {
		result[deal.Status]++
		return false, nil
	})
}
