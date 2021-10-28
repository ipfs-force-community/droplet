package badger

import (
	"bytes"
	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/itf"
	"github.com/ipfs/go-datastore"
	"golang.org/x/xerrors"
)

type retrievalAskRepo struct {
	ds itf.RetrievalAskDS
}

var _ itf.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds itf.RetrievalAskDS) itf.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

func (repo *retrievalAskRepo) GetAsk(addr address.Address) (*retrievalmarket.Ask, error) {
	data, err := repo.ds.Get(statestore.ToKey(addr))
	if err != nil {
		if xerrors.Is(err, datastore.ErrNotFound) {
			err = itf.ErrNotFound
		}
		return nil, err
	}
	var ask retrievalmarket.Ask
	if err = ask.UnmarshalCBOR(bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	return &ask, nil
}

func (repo *retrievalAskRepo) SetAsk(addr address.Address, ask *retrievalmarket.Ask) error {
	data, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}
	return repo.ds.Put(statestore.ToKey(addr), data)
}

func (repo *retrievalAskRepo) Close() error {
	return repo.ds.Close()
}
