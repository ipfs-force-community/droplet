package badger

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

type retrievalAskRepo struct {
	ds RetrievalAskDS
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds RetrievalAskDS) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

func (r *retrievalAskRepo) HasAsk(ctx context.Context, addr address.Address) (bool, error) {
	key := dskeyForAddr(addr)
	return r.ds.Has(ctx, key)
}

func (r *retrievalAskRepo) GetAsk(ctx context.Context, addr address.Address) (*types.RetrievalAsk, error) {
	data, err := r.ds.Get(ctx, dskeyForAddr(addr))
	if err != nil {
		return nil, err
	}
	var ask types.RetrievalAsk
	if err = ask.UnmarshalCBOR(bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	return &ask, nil
}

func (r *retrievalAskRepo) SetAsk(ctx context.Context, ask *types.RetrievalAsk) error {
	data, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}
	return r.ds.Put(ctx, dskeyForAddr(ask.Miner), data)
}

func (r *retrievalAskRepo) ListAsk(ctx context.Context) ([]*types.RetrievalAsk, error) {
	var results []*types.RetrievalAsk
	err := travelDeals(ctx, r.ds, func(ask *types.RetrievalAsk) (bool, error) {
		results = append(results, ask)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
