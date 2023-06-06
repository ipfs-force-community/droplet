package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/libp2p/go-libp2p/core/peer"
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

func prepareRetrievalDealTest(t *testing.T) (context.Context, repo.IRetrievalDealRepo, []types.ProviderDealState) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.RetrievalDealRepo()

	dealCases := make([]types.ProviderDealState, 10)
	testutil.Provide(t, &dealCases)
	return ctx, r, dealCases
}

func TestSaveDeal(t *testing.T) {
	ctx, r, dealCases := prepareRetrievalDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}
}

func TestGetDeal(t *testing.T) {
	ctx, r, dealCases := prepareRetrievalDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	res, err := r.GetDeal(ctx, dealCases[0].Receiver, dealCases[0].ID)
	assert.NoError(t, err)
	dealCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, dealCases[0], *res)
}

func TestGetDealByTransferId(t *testing.T) {
	ctx, r, dealCases := prepareRetrievalDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	res, err := r.GetDealByTransferId(ctx, *dealCases[0].ChannelID)
	assert.NoError(t, err)
	dealCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, dealCases[0], *res)
}

func TestHasDeal(t *testing.T) {
	ctx, r, dealCases := prepareRetrievalDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	dealCase_not_exist := types.ProviderDealState{}
	testutil.Provide(t, &dealCase_not_exist)
	res, err := r.HasDeal(ctx, dealCase_not_exist.Receiver, dealCase_not_exist.ID)
	assert.NoError(t, err)
	assert.False(t, res)

	res, err = r.HasDeal(ctx, dealCases[0].Receiver, dealCases[0].ID)
	assert.NoError(t, err)
	assert.True(t, res)
}

func TestListDeals(t *testing.T) {
	ctx, r, dealCases := prepareRetrievalDealTest(t)

	peerID, err := peer.Decode("12D3KooWCj14HxjJZch7ANeVzmfhPBgZm492euXzoCHiBBEtRsSn")
	assert.NoError(t, err)
	peers := []peer.ID{dealCases[0].Receiver, peerID}
	status := []retrievalmarket.DealStatus{
		retrievalmarket.DealStatusCancelled,
		retrievalmarket.DealStatusAccepted,
		retrievalmarket.DealStatusErrored,
	}

	for i := range dealCases {
		dealCases[i].Receiver = peers[i%2]
		dealCases[i].Status = status[i%3]
		assert.NoError(t, r.SaveDeal(ctx, &dealCases[i]))
	}
	// refresh UpdatedAt
	for i := 0; i < len(dealCases); i++ {
		res, err := r.GetDeal(ctx, dealCases[i].Receiver, dealCases[i].ID)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
	}

	defPage := types.Page{Limit: len(dealCases)}

	// params is empty
	deals, err := r.ListDeals(ctx, &types.RetrievalDealQueryParams{})
	assert.NoError(t, err)
	assert.Len(t, deals, 0)

	// test page
	deals, err = r.ListDeals(ctx, &types.RetrievalDealQueryParams{
		Page: types.Page{
			Limit: 3,
		},
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)
	deals, err = r.ListDeals(ctx, &types.RetrievalDealQueryParams{
		Page: types.Page{
			Offset: len(dealCases) - 3,
			Limit:  4,
		},
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)

	for i := 0; i < 2; i++ {
		deals, err = r.ListDeals(ctx, &types.RetrievalDealQueryParams{
			Receiver: peers[i].Pretty(),
			Page:     defPage,
		})
		assert.NoError(t, err)
		assert.Len(t, deals, 5)
	}

	dealStatusAccepted := uint64(retrievalmarket.DealStatusAccepted)
	deals, err = r.ListDeals(ctx, &types.RetrievalDealQueryParams{
		Status: &dealStatusAccepted,
		Page:   defPage,
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)

	deals, err = r.ListDeals(ctx, &types.RetrievalDealQueryParams{
		DiscardFailedDeal: true,
		Page:              defPage,
	})
	assert.NoError(t, err)
	assert.Equal(t, 7, len(deals))

	dealStatusErrored := uint64(retrievalmarket.DealStatusErrored)
	deals, err = r.ListDeals(ctx, &types.RetrievalDealQueryParams{
		Status:            &dealStatusErrored,
		DiscardFailedDeal: true,
		Page:              defPage,
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)
}

func TestGroupRetrievalDealNumberByStatus(t *testing.T) {
	ctx, r, dealCases := prepareRetrievalDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}
	// refresh UpdatedAt
	for i := 0; i < len(dealCases); i++ {
		res, err := r.GetDeal(ctx, dealCases[i].Receiver, dealCases[i].ID)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
	}

	expect := map[retrievalmarket.DealStatus]int64{}
	for _, deal := range dealCases {
		expect[deal.Status]++
	}
	res, err := r.GroupRetrievalDealNumberByStatus(ctx, address.Undef)
	assert.NoError(t, err)
	assert.Equal(t, expect, res)
}
