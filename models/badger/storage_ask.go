package badger

import (
	"bytes"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"golang.org/x/xerrors"
)

type storageAskRepo struct {
	ds datastore.Batching
}

func NewStorageAskRepo(ds StorageAskDS) *storageAskRepo {
	return &storageAskRepo{ds: ds}
}

func (ar *storageAskRepo) GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	key := statestore.ToKey(miner)
	b, err := ar.ds.Get(key)
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}

	ask := storagemarket.SignedStorageAsk{}
	if err := ask.UnmarshalCBOR(bytes.NewBuffer(b)); err != nil {
		return nil, xerrors.Errorf("bader Miner(%s) unmarshal storageask failed:%w", miner.String(), err)
	}
	return &ask, nil
}

func (ar *storageAskRepo) SetAsk(ask *storagemarket.SignedStorageAsk) error {
	if ask == nil || ask.Ask == nil {
		return xerrors.Errorf("param is nil")
	}
	key := statestore.ToKey(ask.Ask.Miner)
	b, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}

	return ar.ds.Put(key, b)
}

func (ar *storageAskRepo) ListAsk() ([]*storagemarket.SignedStorageAsk, error) {
	var results []*storagemarket.SignedStorageAsk
	err := ar.travelDeals(func(ask *storagemarket.SignedStorageAsk) error {
		results = append(results, ask)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (ar *storageAskRepo) travelDeals(travelFn func(ask *storagemarket.SignedStorageAsk) error) error {
	result, err := ar.ds.Query(query.Query{})
	if err != nil {
		return err
	}
	defer result.Close() //nolint:errcheck
	for res := range result.Next() {
		if res.Error != nil {
			return err
		}
		var ask storagemarket.SignedStorageAsk
		if err = cborrpc.ReadCborRPC(bytes.NewReader(res.Value), &ask); err != nil {
			return err
		}
		if err = travelFn(&ask); err != nil {
			return err
		}
	}
	return nil
}
