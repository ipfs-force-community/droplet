package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/stretchr/testify/assert"
)

func TestSaveFundedAddressState(t *testing.T) {
	ctx, r, fundedAddressStateCases := prepareFundTest(t)

	for _, state := range fundedAddressStateCases {
		err := r.SaveFundedAddressState(ctx, &state)
		assert.NoError(t, err)
	}
}

func TestGetFundedAddressState(t *testing.T) {
	ctx, r, fundedAddressStateCases := prepareFundTest(t)

	for _, state := range fundedAddressStateCases {
		err := r.SaveFundedAddressState(ctx, &state)
		assert.NoError(t, err)
	}

	res, err := r.GetFundedAddressState(ctx, fundedAddressStateCases[0].Addr)
	assert.NoError(t, err)
	fundedAddressStateCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, fundedAddressStateCases[0], *res)
}

func TestListFundedAddressState(t *testing.T) {
	ctx, r, fundedAddressStateCases := prepareFundTest(t)

	for _, state := range fundedAddressStateCases {
		err := r.SaveFundedAddressState(ctx, &state)
		assert.NoError(t, err)
	}

	for i := 0; i < len(fundedAddressStateCases); i++ {
		res, err := r.GetFundedAddressState(ctx, fundedAddressStateCases[i].Addr)
		assert.NoError(t, err)
		fundedAddressStateCases[i].UpdatedAt = res.UpdatedAt
	}

	t.Run("ListFundedAddressState", func(t *testing.T) {
		res, err := r.ListFundedAddressState(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(fundedAddressStateCases), len(res))

		for i := 0; i < len(res); i++ {
			assert.Contains(t, fundedAddressStateCases, *res[i])
		}
	})
}

func prepareFundTest(t *testing.T) (context.Context, repo.FundRepo, []types.FundedAddressState) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.FundRepo()

	fundedAddressStateCases := make([]types.FundedAddressState, 10)
	testutil.Provide(t, &fundedAddressStateCases)
	return ctx, r, fundedAddressStateCases
}
