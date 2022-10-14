package models

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus-market/v2/models/badger"
	t220 "github.com/filecoin-project/venus-market/v2/models/badger/migrate/v2.2.0/testing"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"

	"github.com/stretchr/testify/assert"
)

func TestBadgerMigrate(t *testing.T) {
	var ds datastore.Batching
	var err error
	var count = 3

	var paychMsgCIDs []cid.Cid

	ctx := context.Background()

	ds, err = badger.NewDatastore("")
	assert.NoError(t, err)

	paychMsgCIDs = t220.WriteTestcasesToDS(ctx, t, ds, count)

	repo := badger.WrapDbToRepo(ds)

	assert.NoError(t, repo.Migrate())

	{
		res, err := repo.StorageDealRepo().ListDeal(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)
	}

	{
		for _, cid := range paychMsgCIDs {
			res, err := repo.PaychMsgInfoRepo().GetMessage(ctx, cid)
			assert.NoError(t, err)
			assert.NotNil(t, res)

		}
	}

	{
		res, err := repo.PaychChannelInfoRepo().ListChannel(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)
	}

	{
		res, err := repo.StorageAskRepo().ListAsk(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)
	}
	{
		res, err := repo.RetrievalAskRepo().ListAsk(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)
	}

	{
		res, err := repo.RetrievalDealRepo().ListDeals(ctx, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)

	}

	{
		res, err := repo.CidInfoRepo().ListCidInfoKeys(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)
	}
}
