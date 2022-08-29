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
		count := result[deal.Status]
		count++
		result[deal.Status] = count
	}

	for _, deal := range deals {
		err := r.SaveDeal(ctx, &deal)
		assert.Nil(t, err)
	}

	result2, err := r.GroupRetrievalDealNumberByStatus(ctx, address.Undef)
	assert.Nil(t, err)
	assert.Equal(t, result, result2)
}
