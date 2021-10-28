package models

import (
	"os"
	"testing"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go test -v ./models -test.run TestRetrievalAsk -mysql='root:ko2005@tcp(127.0.0.1:3306)/storage_market?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s'
func TestRetrievalAsk(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		testRetrievalAsk(t, MysqlDB(t).RetrievalAskRepo())
	})

	t.Run("badger", func(t *testing.T) {
		path := "./badger_retrieval_ask_db"
		db := BadgerDB(t, path)
		defer func() {
			assert.Nil(t, db.Close())
			assert.Nil(t, os.RemoveAll(path))

		}()
		testRetrievalAsk(t, badger.NewRetrievalAskRepo(db))
	})
}

func testRetrievalAsk(t *testing.T, rtAskRepo itf.IRetrievalAskRepo) {
	addr := randAddress(t)
	_, err := rtAskRepo.GetAsk(addr)
	assert.ErrorIs(t, err, itf.ErrNotFound, "must be an not found error")

	ask1 := &retrievalmarket.Ask{
		PricePerByte:            abi.NewTokenAmount(1024),
		UnsealPrice:             abi.NewTokenAmount(2048),
		PaymentInterval:         20,
		PaymentIntervalIncrease: 10,
	}

	require.NoError(t, rtAskRepo.SetAsk(addr, ask1))

	var ask2 *retrievalmarket.Ask
	ask2, err = rtAskRepo.GetAsk(addr)

	require.NoError(t, err)
	assert.Equal(t, ask1, ask2)

	newPricePerByte := abi.NewTokenAmount(3045)
	newPaymentInterval := uint64(4000)

	ask1.PricePerByte = newPricePerByte
	ask1.PaymentInterval = newPaymentInterval

	require.NoError(t, rtAskRepo.SetAsk(addr, ask1))
	ask2, err = rtAskRepo.GetAsk(addr)
	assert.Nil(t, err)
	assert.Equal(t, ask1, ask2)
}
