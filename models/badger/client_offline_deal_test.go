package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus-market/v2/types"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestClientOfflineDeal(t *testing.T) {
	ds, err := NewDatastore("")
	assert.NoError(t, err)
	r := NewBadgerClientOfflineDealRepo(ds)

	var deals []*types.ClientOfflineDeal
	testutil.Provide(t, &deals)

	ctx := context.Background()

	t.Run("save deal", func(t *testing.T) {
		for _, deal := range deals {
			assert.NoError(t, r.SaveDeal(ctx, deal))
		}
	})

	t.Run("get deal", func(t *testing.T) {
		for _, deal := range deals {
			res, err := r.GetDeal(ctx, deal.ProposalCID)
			assert.NoError(t, err)
			labelByte, err := deal.Proposal.Label.ToBytes()
			assert.NoError(t, err)
			labelStr, err := res.Proposal.Label.ToString()
			assert.NoError(t, err)
			assert.Equal(t, string(labelByte), labelStr)
			res.Proposal.Label, err = vTypes.NewLabelFromBytes([]byte(labelStr))
			assert.NoError(t, err)
			assert.Equal(t, deal, res)
		}
	})

	t.Run("list deal", func(t *testing.T) {
		res, err := r.ListDeal(ctx)
		assert.NoError(t, err)

		assert.Len(t, res, len(deals))
	})
}
