package StorageAsk

import (
	"bytes"

	"github.com/dgraph-io/badger/v2"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"golang.org/x/xerrors"
)

type badgerStorageAsk struct {
	ds *badger.DB
}

var _ istorageAskRepo = (*badgerStorageAsk)(nil)

func (b *badgerStorageAsk) GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	var data []byte

	if err := b.ds.View(func(txn *badger.Txn) error {
		item, err := txn.Get(miner.Bytes())
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	}); err != nil {
		return nil, xerrors.Errorf("badger get Miner(%s) storageask failed:%w", miner.String(), err)
	}

	var ask = &storagemarket.SignedStorageAsk{}
	if err := ask.UnmarshalCBOR(bytes.NewBuffer(data)); err != nil {
		return nil, xerrors.Errorf("bader Miner(%s) unmarshal storageask failed:%w", miner.String(), err)
	}
	return ask, nil
}

func (b *badgerStorageAsk) SetAsk(miner address.Address, ask *storagemarket.SignedStorageAsk) error {
	buf := bytes.NewBuffer(nil)
	if err := ask.MarshalCBOR(buf); err != nil {
		return xerrors.Errorf("bader set Miner(%s) ask, marshal SignedAsk failed:%w", err)
	}

	if err := b.ds.Update(func(txn *badger.Txn) error {
		return txn.Set(miner.Bytes(), buf.Bytes())
	}); err != nil {
		return xerrors.Errorf("badger set ask, update failed:%w", err)
	}
	return nil
}

func (b *badgerStorageAsk) Close() error {
	return b.ds.Close()
}

func newBadgerStorageAskRepo(cfg *StorageAskCfg) (*badgerStorageAsk, error) {
	ds, err := badger.Open(badger.DefaultOptions(cfg.URI))
	if err != nil {
		return nil, xerrors.Errorf("open badger(%s) failed:%w", cfg.URI, err)
	}
	return &badgerStorageAsk{ds: ds}, nil
}
