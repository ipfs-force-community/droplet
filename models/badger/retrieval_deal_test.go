package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/assert"
)

func init() {
	testutil.MustRegisterDefaultValueProvier(func(t *testing.T) *cbg.Deferred {
		return &cbg.Deferred{
			Raw: make([]byte, 1),
		}
	})
}

func TestRetrievalDeal(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.RetrievalDealRepo()

	dealCases := make([]types.ProviderDealState, 10)
	testutil.Provide(t, &dealCases)

	t.Run("SaveDeal", func(t *testing.T) {
		for _, deal := range dealCases {
			err := r.SaveDeal(ctx, &deal)
			assert.NoError(t, err)
		}
	})

	t.Run("GetDeal", func(t *testing.T) {
		res, err := r.GetDeal(ctx, dealCases[0].Receiver, dealCases[0].ID)
		assert.NoError(t, err)
		dealCases[0].UpdatedAt = res.UpdatedAt
		assert.Equal(t, dealCases[0], *res)
	})

	t.Run("GetDealByTransferId", func(t *testing.T) {
		res, err := r.GetDealByTransferId(ctx, *dealCases[0].ChannelID)
		assert.NoError(t, err)
		dealCases[0].UpdatedAt = res.UpdatedAt
		assert.Equal(t, dealCases[0], *res)
	})

	t.Run("HasDeal", func(t *testing.T) {
		dealCase_not_exist := types.ProviderDealState{}
		testutil.Provide(t, &dealCase_not_exist)
		res, err := r.HasDeal(ctx, dealCase_not_exist.Receiver, dealCase_not_exist.ID)
		assert.NoError(t, err)
		assert.False(t, res)

		res, err = r.HasDeal(ctx, dealCases[0].Receiver, dealCases[0].ID)
		assert.NoError(t, err)
		assert.True(t, res)
	})

	// refresh UpdatedAt
	for i := 0; i < len(dealCases); i++ {
		res, err := r.GetDeal(ctx, dealCases[i].Receiver, dealCases[i].ID)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
	}

	t.Run("ListDeals", func(t *testing.T) {
		res, err := r.ListDeals(ctx, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, len(dealCases), len(res))
		for _, res := range res {
			assert.Contains(t, dealCases, *res)
		}
	})

	t.Run("GroupRetrievalDealNumberByStatus", func(t *testing.T) {
		expect := map[retrievalmarket.DealStatus]int64{}
		for _, deal := range dealCases {
			expect[deal.Status]++
		}
		res, err := r.GroupRetrievalDealNumberByStatus(ctx, address.Undef)
		assert.NoError(t, err)
		assert.Equal(t, expect, res)
	})
}
