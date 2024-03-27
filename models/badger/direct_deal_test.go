package badger

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
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
				res, err := r.GetDealByAllocationID(ctx, deal.AllocationID)
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

		res, err = r.ListDeal(ctx, types.DirectDealQueryParams{
			Page: types.Page{
				Offset: 0,
				Limit:  10,
			},
		})
		assert.NoError(t, err)
		assert.Len(t, res, 10)
	})
}

func prepareDirectDealTest(t *testing.T) (context.Context, repo.DirectDealRepo, []types.DirectDeal) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.DirectDealRepo()

	dealCases := make([]types.DirectDeal, 10)
	testutil.Provide(t, &dealCases)
	dealCases[0].State = types.DealAllocated
	return ctx, r, dealCases
}

func TestGetDirectDealPieceInfo(t *testing.T) {
	ctx, r, dealCases := prepareDirectDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ID)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
	}

	res, err := r.GetPieceInfo(ctx, dealCases[0].PieceCID)
	assert.NoError(t, err)
	expect := piecestore.PieceInfo{
		PieceCID: dealCases[0].PieceCID,
		Deals:    nil,
	}
	expect.Deals = append(expect.Deals, piecestore.DealInfo{
		SectorID: dealCases[0].SectorID,
		Offset:   dealCases[0].Offset,
		Length:   dealCases[0].PieceSize,
	})
	assert.Equal(t, expect, *res)
}

func TestGetDirectDealPieceSize(t *testing.T) {
	ctx, r, dealCases := prepareDirectDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ID)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
	}

	PLSize, PSize, err := r.GetPieceSize(ctx, dealCases[0].PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, dealCases[0].PieceSize, PSize)
	assert.Equal(t, dealCases[0].PayloadSize, PLSize)
}
