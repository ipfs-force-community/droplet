package badger

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-datastore"
)

type storageAskRepo struct {
	ds datastore.Batching
}

func NewStorageAskRepo(ds StorageAskDS) repo.IStorageAskRepo {
	return &storageAskRepo{ds: ds}
}

func (ar *storageAskRepo) GetAsk(ctx context.Context, miner address.Address) (*types.SignedStorageAsk, error) {
	key := statestore.ToKey(miner)
	b, err := ar.ds.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	ask := types.SignedStorageAsk{}
	if err := ask.UnmarshalCBOR(bytes.NewBuffer(b)); err != nil {
		return nil, fmt.Errorf("bader Miner(%s) unmarshal storageask failed:%w", miner.String(), err)
	}
	return &ask, nil
}

func (ar *storageAskRepo) SetAsk(ctx context.Context, ask *types.SignedStorageAsk) error {
	if ask == nil || ask.Ask == nil {
		return fmt.Errorf("param is nil")
	}
	// This method is generally called from command tool `storage-deals set-ask`
	// the input arguments doesn't have old `Timestamp`, we try to get the older for updatting
	oldAsk, err := ar.GetAsk(ctx, ask.Ask.Miner)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			return err
		}
	} else {
		ask.TimeStamp = oldAsk.TimeStamp
	}

	ask.TimeStamp = makeRefreshedTimeStamp(&ask.TimeStamp)

	key := statestore.ToKey(ask.Ask.Miner)
	b, err := cborrpc.Dump(ask)
	if err != nil {
		return err
	}

	return ar.ds.Put(ctx, key, b)
}

func (ar *storageAskRepo) ListAsk(ctx context.Context) ([]*types.SignedStorageAsk, error) {
	var results []*types.SignedStorageAsk
	err := travelDeals(ctx, ar.ds, func(ask *types.SignedStorageAsk) (bool, error) {
		results = append(results, ask)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
