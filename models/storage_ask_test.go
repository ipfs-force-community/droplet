package models

import (
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/itf"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestStorageAsk(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		testStorageAsk(t, MysqlDB(t).StorageAskRepo())
	})

	t.Run("badger", func(t *testing.T) {
		path := "./badger_stoarage_ask_db"
		db := BadgerDB(t, path)
		defer func() {
			assert.Nil(t, db.Close())
			assert.Nil(t, os.RemoveAll(path))

		}()
		testStorageAsk(t, itf.IStorageAskRepo(badger.NewAskStore(db)))
	})
}
func testStorageAsk(t *testing.T, askRepo itf.IStorageAskRepo) {
	ask := &storagemarket.SignedStorageAsk{
		Ask: &storagemarket.StorageAsk{
			Price:         abi.NewTokenAmount(10),
			VerifiedPrice: abi.NewTokenAmount(100),
			MinPieceSize:  1024,
			MaxPieceSize:  1024,
			Miner:         randAddress(t),
			Timestamp:     abi.ChainEpoch(10),
			Expiry:        abi.ChainEpoch(10),
			SeqNo:         0,
		},
		Signature: nil,
	}

	ask2 := &storagemarket.SignedStorageAsk{
		Ask: &storagemarket.StorageAsk{
			Price:         abi.NewTokenAmount(10),
			VerifiedPrice: abi.NewTokenAmount(100),
			MinPieceSize:  1024,
			MaxPieceSize:  1024,
			Miner:         randAddress(t),
			Timestamp:     abi.ChainEpoch(10),
			Expiry:        abi.ChainEpoch(10),
			SeqNo:         0,
		},
		Signature: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte("bls"),
		},
	}

	assert.Nil(t, askRepo.SetAsk(ask))
	assert.Nil(t, askRepo.SetAsk(ask2))

	res, err := askRepo.GetAsk(ask.Ask.Miner)
	assert.Nil(t, err)
	compareStorageAsk(t, res, ask)
	res2, err := askRepo.GetAsk(ask2.Ask.Miner)
	assert.Nil(t, err)
	compareStorageAsk(t, res2, ask2)
}

func compareStorageAsk(t *testing.T, actual, expected *storagemarket.SignedStorageAsk) {
	assert.Equal(t, expected.Ask, actual.Ask)
	assert.Equal(t, expected.Signature, actual.Signature)
}
