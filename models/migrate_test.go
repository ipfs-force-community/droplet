package models

import (
	"context"
	"math"
	"testing"

	cbor "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/badger"
	t220 "github.com/ipfs-force-community/droplet/v2/models/badger/migrate/v2.2.0/testing"
	v230 "github.com/ipfs-force-community/droplet/v2/models/badger/migrate/v2.3.0"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
)

func TestBadgerMigrate(t *testing.T) {
	var ds datastore.Batching
	var err error
	count := 3

	var paychMsgCIDs []cid.Cid

	ctx := context.Background()

	ds, err = badger.NewDatastore("")
	assert.NoError(t, err)

	paychMsgCIDs = t220.WriteTestcasesToDS(ctx, t, ds, count)

	repo := badger.WrapDbToRepo(ds)

	assert.NoError(t, repo.Migrate())

	{
		res, err := repo.StorageDealRepo().ListDeal(ctx, &market.StorageDealQueryParams{Page: market.Page{Limit: math.MaxInt32}})
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
		res, err := repo.RetrievalDealRepo().ListDeals(ctx, &market.RetrievalDealQueryParams{Page: market.Page{Limit: 10}})
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)

	}

	{
		res, err := repo.CidInfoRepo().ListCidInfoKeys(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(res), count)
	}
}

func TestBadgerMigrateV230(t *testing.T) {
	ctx := context.Background()

	ds, err := badger.NewDatastore("")
	assert.NoError(t, err)

	deals := make([]*v230.MinerDeal, 10)
	testutil.Provide(t, &deals)

	for _, deal := range deals {
		data, err := cbor.Dump(deal)
		assert.NoError(t, err)
		assert.NoError(t, ds.Put(ctx, deal.KeyWithNamespace(), data))
	}

	// write version
	assert.NoError(t, ds.Put(ctx, datastore.NewKey("/storage/provider/deals").ChildString("/versions/current"), []byte("1")))

	repo := badger.WrapDbToRepo(ds)
	assert.NoError(t, repo.Migrate())

	res, err := repo.StorageDealRepo().ListDeal(ctx, &market.StorageDealQueryParams{Page: market.Page{Limit: math.MaxInt32}})
	assert.NoError(t, err)
	assert.Equal(t, len(res), len(deals))
	for _, deal := range res {
		t.Log("id: ", deal)
		assert.NotEmpty(t, deal.ID)
	}
}
