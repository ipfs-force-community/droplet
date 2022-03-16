package models

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/repo"
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

	assert.Nil(t, askRepo.SetAsk(ctx, ask))
	assert.Nil(t, askRepo.SetAsk(ctx, ask2))

	res, err := askRepo.GetAsk(ctx, ask.Ask.Miner)
	assert.Nil(t, err)
	assert.Equal(t, res, ask)
	res2, err := askRepo.GetAsk(ctx, ask2.Ask.Miner)
	assert.Nil(t, err)
	assert.Equal(t, res2, ask2)
}
