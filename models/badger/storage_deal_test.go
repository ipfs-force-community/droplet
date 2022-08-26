package badger

import (
	"context"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/filecoin-project/go-address"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func Test_storageDealRepo_GroupDealsByStatus(t *testing.T) {
	ctx := context.Background()
	t.Run("correct", func(t *testing.T) {
		repo := setup(t)
		r := repo.StorageDealRepo()

		deals := make([]market.MinerDeal, 100, 100)
		testutil.Provide(t, &deals)

		var addrs []address.Address
		addrGetter := address.NewForTestGetter()
		for i := 0; i < 3; i++ {
			addrs = append(addrs, addrGetter())
		}

		for index := range deals {
			deals[index].ClientDealProposal.Proposal.Provider = addrs[rand.Intn(len(addrs))]
			deals[index].State = storagemarket.StorageDealStatus(rand.Intn(int(storagemarket.StorageDealReserveProviderFunds)))
		}

		for _, deal := range deals {
			err := r.SaveDeal(ctx, &deal)
			assert.Nil(t, err)
		}

		result := map[storagemarket.StorageDealStatus]int64{}
		for _, deal := range deals {
			if deal.Proposal.Provider != addrs[0] {
				continue
			}
			count := result[deal.State]
			count++
			result[deal.State] = count
		}
		result2, err := r.GroupStorageDealNumberByStatus(ctx, addrs[0])
		assert.Nil(t, err)
		assert.Equal(t, result, result2)
	})

	t.Run("undefined address", func(t *testing.T) {
		repo := setup(t)
		r := repo.StorageDealRepo()

		deals := make([]market.MinerDeal, 10, 10)
		testutil.Provide(t, &deals)

		result := map[storagemarket.StorageDealStatus]int64{}
		for _, deal := range deals {
			count := result[deal.State]
			count++
			result[deal.State] = count
		}

		for _, deal := range deals {
			err := r.SaveDeal(ctx, &deal)
			assert.Nil(t, err)
		}

		result2, err := r.GroupStorageDealNumberByStatus(ctx, address.Undef)
		assert.Nil(t, err)
		assert.Equal(t, result, result2)
	})
}
