package models

import (
	"context"
	"testing"

	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go test -v ./models -test.run TestRetrievalAsk -mysql='root:ko2005@tcp(127.0.0.1:3306)/storage_market?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s'
func TestRetrievalAsk(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		repo := MysqlDB(t)
		retrievalAskRepo := repo.RetrievalAskRepo()
		defer func() { require.NoError(t, repo.Close()) }()
		testRetrievalAsk(t, retrievalAskRepo)
	})

	t.Run("badger", func(t *testing.T) {
		db := BadgerDB(t)
		testRetrievalAsk(t, badger.NewRetrievalAskRepo(db))
	})
}

func testRetrievalAsk(t *testing.T, rtAskRepo repo.IRetrievalAskRepo) {
	ctx := context.Background()
	addr := randAddress(t)
	_, err := rtAskRepo.GetAsk(ctx, addr)
	assert.Equal(t, err.Error(), repo.ErrNotFound.Error(), "must be an not found error")

	ask := &types.RetrievalAsk{
		Miner:                   addr,
		PricePerByte:            abi.NewTokenAmount(1024),
		UnsealPrice:             abi.NewTokenAmount(2048),
		PaymentInterval:         20,
		PaymentIntervalIncrease: 10,
	}
	require.NoError(t, rtAskRepo.SetAsk(ctx, ask))

	ask2, err := rtAskRepo.GetAsk(ctx, addr)
	require.NoError(t, err)
	assert.Equal(t, ask, ask2)

	ask.PricePerByte = abi.NewTokenAmount(1000)
	ask.UnsealPrice = abi.NewTokenAmount(1000)
	ask.PaymentInterval = 1000
	ask.PaymentIntervalIncrease = 1000

	require.NoError(t, rtAskRepo.SetAsk(ctx, ask))
	ask2, err = rtAskRepo.GetAsk(ctx, addr)
	assert.Nil(t, err)
	assert.Equal(t, ask, ask2)
}
