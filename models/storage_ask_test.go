package models

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-state-types/big"
	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageAsk(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		repo := MysqlDB(t)
		askRepo := repo.StorageAskRepo()
		defer func() { require.NoError(t, repo.Close()) }()
		testStorageAsk(t, askRepo)
	})
	t.Run("badger", func(t *testing.T) {
		db := BadgerDB(t)
		testStorageAsk(t, badger.NewStorageAskRepo(db))
	})
}

func testStorageAsk(t *testing.T, askRepo repo.IStorageAskRepo) {
	ctx := context.Background()
	ask := &types.SignedStorageAsk{
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

	ask2 := &types.SignedStorageAsk{
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

	assertEqual := func(s1, s2 *types.SignedStorageAsk) {
		assert.Equal(t, s1.Ask, s2.Ask)
		assert.Equal(t, s1.Signature, s2.Signature)
		assert.Equal(t, s1.CreatedAt, s2.CreatedAt)
	}

	assert.Nil(t, askRepo.SetAsk(ctx, ask))
	assert.Nil(t, askRepo.SetAsk(ctx, ask2))

	res, err := askRepo.GetAsk(ctx, ask.Ask.Miner)
	assert.Nil(t, err)
	assertEqual(res, ask)
	res2, err := askRepo.GetAsk(ctx, ask2.Ask.Miner)
	assert.Nil(t, err)
	assertEqual(res2, ask2)

	newPrice := big.Add(ask.Ask.Price, abi.NewTokenAmount(1))

	// updating storage-ask timestamp test
	tmpAsk := *ask
	tmpAsk.Ask.Price = newPrice

	// to simulate updating storage-ask with zero timestamp.
	tmpAsk.TimeStamp = types.TimeStamp{}
	assert.Nil(t, askRepo.SetAsk(ctx, &tmpAsk))
	res3, err := askRepo.GetAsk(ctx, ask.Ask.Miner)
	assert.Nil(t, err)

	assert.Equal(t, big.Cmp(res3.Ask.Price, newPrice), 0)
	assert.Equal(t, ask.CreatedAt, res.CreatedAt)
	assert.GreaterOrEqual(t, res3.UpdatedAt, res3.CreatedAt)
	assert.GreaterOrEqual(t, res3.UpdatedAt, res.CreatedAt)
}
