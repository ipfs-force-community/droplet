package badger

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDirectDeal(t *testing.T) {
	ds, err := NewDatastore("")
	assert.NoError(t, err)
	r := NewDirectDealRepo(ds)

	deals := make([]*types.DirectDeal, 10)
	testutil.Provide(t, &deals)

	ctx := context.Background()

	t.Run("save deal", func(t *testing.T) {
		for _, deal := range deals {
			assert.NoError(t, r.SaveDeal(ctx, deal))
		}
	})

	t.Run("get deal", func(t *testing.T) {
		for _, deal := range deals {
			res, err := r.GetDeal(ctx, deal.ID)
			assert.NoError(t, err)
			assert.Equal(t, deal, res)
		}

		res, err := r.GetDeal(ctx, uuid.New())
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("get deal by allocation id", func(t *testing.T) {
		for _, deal := range deals {
			if deal.State != types.DealError {
				res, err := r.GetDealByAllocationID(ctx, uint64(deal.AllocationID))
				assert.NoError(t, err)
				assert.Equal(t, deal, res)
			}

		}

		res, err := r.GetDealByAllocationID(ctx, uint64(time.Now().Unix()))
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("list deal", func(t *testing.T) {
		res, err := r.ListDeal(ctx)
		assert.NoError(t, err)

		assert.Len(t, res, len(deals))
	})
}
