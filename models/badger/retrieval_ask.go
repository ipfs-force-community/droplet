package badger

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-datastore"
	"golang.org/x/xerrors"
)

type retrievalAskRepo struct {
	ds repo.RetrievalAskDS
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds repo.RetrievalAskDS) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

func (r *retrievalAskRepo) GetAsk(addr address.Address) (*retrievalmarket.Ask, error) {
	data, err := r.ds.Get(statestore.ToKey(addr))
	if err != nil {
		if xerrors.Is(err, datastore.ErrNotFound) {
			err = repo.ErrNotFound
		}
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

func (r *retrievalAskRepo) Close() error {
	return r.ds.Close()
}
