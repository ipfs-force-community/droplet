package badger

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/types"
)

type retrievalAskRepo struct {
	ds RetrievalAskDS
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds RetrievalAskDS) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

func (r *retrievalAskRepo) HasAsk(addr address.Address) (bool, error) {
	key := dskeyForAddr(addr)
	return r.ds.Has(key)
}

func (r *retrievalAskRepo) GetAsk(addr address.Address) (*types.RetrievalAsk, error) {
	data, err := r.ds.Get(dskeyForAddr(addr))
	if err != nil {
		return nil, err
	}
	var ask types.RetrievalAsk
	if err = ask.UnmarshalCBOR(bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	return &ask, nil
}

func (r *retrievalAskRepo) SetAsk(ask *types.RetrievalAsk) error {
	data, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}
	return r.ds.Put(dskeyForAddr(ask.Miner), data)
}

func (r *retrievalAskRepo) ListAsk() ([]*types.RetrievalAsk, error) {
	var results []*types.RetrievalAsk
	err := travelDeals(r.ds, func(ask *types.RetrievalAsk) (bool, error) {
		results = append(results, ask)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}