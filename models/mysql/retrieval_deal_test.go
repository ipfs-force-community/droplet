package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/stretchr/testify/assert"
)

func Test_retrievalDealRepo_GroupRetrievalDealNumberByStatus(t *testing.T) {
	ctx := context.Background()
	r, mock, _ := setup(t)
	expectResult := map[retrievalmarket.DealStatus]int64{
		retrievalmarket.DealStatusAccepted:       1,
		retrievalmarket.DealStatusErrored:        1432,
		retrievalmarket.DealStatusCompleted:      13,
		retrievalmarket.DealStatusBlocksComplete: 100,
	}
	rows := mock.NewRows([]string{"state", "count"})
	for status, count := range expectResult {
		rows.AddRow(status, count)
	}

	addr, err := address.NewIDAddress(10)
	assert.Nil(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT state, count(1) as count FROM `retrieval_deals` GROUP BY `state`")).WillReturnRows(rows)
	result, err := r.RetrievalDealRepo().GroupRetrievalDealNumberByStatus(ctx, addr)
	assert.Nil(t, err)
	assert.Equal(t, expectResult, result)
}
