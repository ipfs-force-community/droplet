package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/assert"
)

func TestStorageAsk(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.StorageAskRepo()

	askCases := make([]types.SignedStorageAsk, 10)
	testutil.Provide(t, &askCases)

	t.Run("SetAsk", func(t *testing.T) {
		for _, ask := range askCases {
			err := r.SetAsk(ctx, &ask)
			assert.NoError(t, err)
		}
	})

	t.Run("GetAsk", func(t *testing.T) {
		res, err := r.GetAsk(ctx, askCases[0].Ask.Miner)
		assert.NoError(t, err)
		askCases[0].UpdatedAt = res.UpdatedAt
		assert.Equal(t, askCases[0], *res)
	})

	// refresh UpdatedAt field

	for i := 0; i < len(askCases); i++ {
		res, err := r.GetAsk(ctx, askCases[i].Ask.Miner)
		assert.NoError(t, err)
		askCases[i].UpdatedAt = res.UpdatedAt
	}

	t.Run("ListAsk", func(t *testing.T) {
		res, err := r.ListAsk(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(askCases), len(res))
		for _, ask := range res {
			assert.Contains(t, askCases, *ask)
		}
	})
}
