package badger

import (
	"bytes"
	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
)

type retrievalAskRepo struct {
	ds RetrievalAskDS
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds RetrievalAskDS) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

func (r *retrievalAskRepo) HasAsk(addr address.Address) bool {
	panic("implement me")
}

func (r *retrievalAskRepo) GetAsk(addr address.Address) (*retrievalmarket.Ask, error) {
	data, err := r.ds.Get(statestore.ToKey(addr))
	if err != nil {
		return nil, err
	}
	var ask retrievalmarket.Ask
	if err = ask.UnmarshalCBOR(bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	return &ask, nil
}

func (r *retrievalAskRepo) SetAsk(addr address.Address, ask *retrievalmarket.Ask) error {
	data, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}
	return r.ds.Put(statestore.ToKey(addr), data)
}
