package badger

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
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

	t.Run("get deal by miner and state", func(t *testing.T) {
		addr, err := address.NewIDAddress(1000)
		assert.NoError(t, err)
		for idx := range deals {
			if idx%2 == 0 {
				deals[idx].Provider = addr
				deals[idx].State = types.DealActive
			} else if deals[idx].Provider == addr {
				deals[idx].Provider = address.TestAddress
			}
			assert.NoError(t, r.SaveDeal(ctx, deals[idx]))
		}

		res, err := r.GetDealsByMinerAndState(ctx, addr, types.DealActive)
		assert.NoError(t, err)
		assert.Len(t, res, len(deals)/2)

		res, err = r.GetDealsByMinerAndState(ctx, address.Undef, types.DealActive)
		assert.NoError(t, err)
		assert.Len(t, res, 0)
	})

	t.Run("list deal", func(t *testing.T) {
		var err error
		firstDeal := deals[0]
		firstDeal.State = types.DealError
		firstDeal.Provider, err = address.NewIDAddress(uint64(time.Now().Unix()))
		assert.NoError(t, err)
		firstDeal.Client, err = address.NewIDAddress(uint64(time.Now().Unix() + 10))
		assert.NoError(t, err)
		assert.NoError(t, r.SaveDeal(ctx, firstDeal))
		params := types.DirectDealQueryParams{
			Provider: firstDeal.Provider,
			Client:   firstDeal.Client,
			State:    &firstDeal.State,
			Page: types.Page{
				Offset: 0,
				Limit:  10,
			},
		}
		res, err := r.ListDeal(ctx, params)
		assert.NoError(t, err)

		assert.Len(t, res, 1)
	})
}
