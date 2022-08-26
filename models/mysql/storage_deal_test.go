package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-address"
)

func Test_storageDealRepo_GroupDealsByStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("correct", func(t *testing.T) {
		r, mock, _ := setup(t)
		expectResult := map[storagemarket.StorageDealStatus]int64{
			storagemarket.StorageDealActive: 1,
		}
		rows := mock.NewRows([]string{"state", "count"})
		for status, count := range expectResult {
			rows.AddRow(status, count)
		}

		addr, err := address.NewIDAddress(10)
		assert.Nil(t, err)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT state, count(1) as count FROM `storage_deals` WHERE cdp_provider = ? GROUP BY `state`")).WithArgs(DBAddress(addr).String()).WillReturnRows(rows)
		result, err := r.StorageDealRepo().GroupStorageDealNumberByStatus(ctx, addr)
		assert.Nil(t, err)
		assert.Equal(t, expectResult, result)
	})

	t.Run("undefined address", func(t *testing.T) {
		r, mock, _ := setup(t)
		expectResult := map[storagemarket.StorageDealStatus]int64{
			storagemarket.StorageDealActive: 1,
		}
		rows := mock.NewRows([]string{"state", "count"})
		for status, count := range expectResult {
			rows.AddRow(status, count)
		}

		mock.ExpectQuery(regexp.QuoteMeta("SELECT state, count(1) as count FROM `storage_deals` GROUP BY `state`")).WillReturnRows(rows)
		result, err := r.StorageDealRepo().GroupStorageDealNumberByStatus(ctx, address.Undef)
		assert.Nil(t, err)
		assert.Equal(t, expectResult, result)
	})

}
