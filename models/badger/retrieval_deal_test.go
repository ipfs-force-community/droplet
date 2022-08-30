package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/assert"
)

func Test_retrievalDealRepo_GroupRetrievalDealNumberByStatus(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.RetrievalDealRepo()

	deals := make([]market.ProviderDealState, 10)
	testutil.Provide(t, &deals)

	result := map[retrievalmarket.DealStatus]int64{}
	for index, deal := range deals {
		deals[index].Params.Selector = nil
		result[deal.Status]++
	}

	for _, deal := range deals {
		err := r.SaveDeal(ctx, &deal)
		assert.Nil(t, err)
	}

	result2, err := r.GroupRetrievalDealNumberByStatus(ctx, address.Undef)
	assert.Nil(t, err)
	assert.Equal(t, result, result2)
}

func Test_retrievalDealRepo_ListDeals(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.RetrievalDealRepo()

	deals := make([]*market.ProviderDealState, 10)
	testutil.Provide(t, &deals)

	for index := range deals {
		deals[index].Params.Selector = nil
	}

	for _, deal := range deals {
		err := r.SaveDeal(ctx, deal)
		assert.Nil(t, err)
	}

	dealInDb, err := r.ListDeals(ctx, 1, 10)
	assert.Nil(t, err)
	assert.Len(t, dealInDb, 10)

	result2, err := r.ListDeals(ctx, 1, 2)
	assert.Nil(t, err)
	assert.Equal(t, dealInDb[:2], result2)

	result2, err = r.ListDeals(ctx, 2, 2)
	assert.Nil(t, err)
	assert.Equal(t, dealInDb[2:4], result2)
}
