package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/stretchr/testify/assert"
)

func prepareRetrievalAskTest(t *testing.T) (context.Context, repo.IRetrievalAskRepo, []types.RetrievalAsk) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.RetrievalAskRepo()

	askCases := make([]types.RetrievalAsk, 10)
	testutil.Provide(t, &askCases)

	return ctx, r, askCases
}

func TestSetAsk(t *testing.T) {
	ctx, r, askCases := prepareRetrievalAskTest(t)

	for _, ask := range askCases {
		err := r.SetAsk(ctx, &ask)
		assert.NoError(t, err)
	}
}

func TestGetAsk(t *testing.T) {
	ctx, r, askCases := prepareRetrievalAskTest(t)

	for _, ask := range askCases {
		err := r.SetAsk(ctx, &ask)
		assert.NoError(t, err)
	}

	res, err := r.GetAsk(ctx, askCases[0].Miner)
	assert.NoError(t, err)
	askCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, askCases[0], *res)
}

func TestListAsk(t *testing.T) {
	ctx, r, askCases := prepareRetrievalAskTest(t)

	for _, ask := range askCases {
		err := r.SetAsk(ctx, &ask)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt field
	for i := 0; i < len(askCases); i++ {
		res, err := r.GetAsk(ctx, askCases[i].Miner)
		assert.NoError(t, err)
		askCases[i].UpdatedAt = res.UpdatedAt
	}

	res, err := r.ListAsk(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(askCases), len(res))
	for _, ask := range res {
		assert.Contains(t, askCases, *ask)
	}
}
