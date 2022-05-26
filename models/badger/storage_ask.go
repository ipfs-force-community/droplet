package badger

import (
	"bytes"
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/ipfs/go-datastore"
)

type storageAskRepo struct {
	ds datastore.Batching
}

func NewStorageAskRepo(ds StorageAskDS) repo.IStorageAskRepo {
	return &storageAskRepo{ds: ds}
}

func (ar *storageAskRepo) GetAsk(ctx context.Context, miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	key := statestore.ToKey(miner)
	b, err := ar.ds.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	ask := storagemarket.SignedStorageAsk{}
	if err := ask.UnmarshalCBOR(bytes.NewBuffer(b)); err != nil {
		return nil, fmt.Errorf("bader Miner(%s) unmarshal storageask failed:%w", miner.String(), err)
	}
	return &ask, nil
}

func (ar *storageAskRepo) SetAsk(ctx context.Context, ask *storagemarket.SignedStorageAsk) error {
	if ask == nil || ask.Ask == nil {
		return fmt.Errorf("param is nil")
	}
	key := statestore.ToKey(ask.Ask.Miner)
	b, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}

	return ar.ds.Put(ctx, key, b)
}

func (ar *storageAskRepo) ListAsk(ctx context.Context) ([]*storagemarket.SignedStorageAsk, error) {
	var results []*storagemarket.SignedStorageAsk
	err := travelDeals(ctx, ar.ds, func(ask *storagemarket.SignedStorageAsk) (bool, error) {
		results = append(results, ask)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
