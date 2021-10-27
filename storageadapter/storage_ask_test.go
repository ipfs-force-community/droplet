package storageadapter

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/utils/test_helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

 // go test -v ./storageadapter -test.run TestStorageAsk -mysql='root:ko2005@tcp(127.0.0.1:3306)/storage_market?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s'
func TestStorageAsk(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		mysqlAsk := &StorageAsk{repo: models.MysqlDB(t).StorageAskRepo(),
			fullNode: test_helper.MockFullnode{t}}
		testStorageAsk(t, mysqlAsk)
	})
	t.Run("badger", func(t *testing.T) {
		path := "./badger_stoarage_ask_db"
		badgerAsk := &StorageAsk{repo: badger.NewAskStore(models.BadgerDB(t, path)),
			fullNode: test_helper.MockFullnode{t}}
		defer func() {
			assert.Nil(t, badgerAsk.Close())
			assert.Nil(t, os.RemoveAll(path))

		}()
		testStorageAsk(t, badgerAsk)
	})
}

func testStorageAsk(t *testing.T, repo *StorageAsk) {
	miner, _ := address.NewFromString("f02438")
	price := abi.NewTokenAmount(100)
	verifyPrice := abi.NewTokenAmount(10333)
	dur := abi.ChainEpoch(10000)

	ask := &storagemarket.StorageAsk{
		Price:         price,
		VerifiedPrice: verifyPrice,
		Miner:         miner,
	}

	require.NoError(t, repo.SetAsk(miner, ask.Price, ask.VerifiedPrice, dur))

	ask2, err := repo.GetAsk(miner)
	require.NoError(t, err)

	require.Equal(t, ask2.Ask.Miner, miner, "miner should equals : %s", miner.String())
	require.Equal(t, ask2.Ask.Price, price, "price should equals : %s", price.String())

	price = big.Add(price, abi.NewTokenAmount(10000))
	verifyPrice = big.Add(verifyPrice, abi.NewTokenAmount(44))

	ask.Price = price
	ask.VerifiedPrice = verifyPrice

	require.NoError(t, repo.SetAsk(miner, ask.Price, ask.VerifiedPrice, dur))

	ask2, err = repo.GetAsk(miner)
	require.NoError(t, err)

	require.Equal(t, ask2.Ask.Price, price, "price should equals : %s", price.String())
	require.Equal(t, ask2.Ask.VerifiedPrice, verifyPrice, "price should equals : %s", verifyPrice.String())
}
