package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/stretchr/testify/assert"
)

var storageAskCase []storagemarket.SignedStorageAsk

func init() {
	addr1, err := address.NewIDAddress(10)
	addr2, err := address.NewIDAddress(20)
	if err != nil {
		panic(err)
	}

	storageAskCase = []storagemarket.SignedStorageAsk{
		{
			Ask: &storagemarket.StorageAsk{
				Miner:         addr1,
				Price:         big.NewInt(0),
				VerifiedPrice: big.NewInt(0),
			},
		},
		{
			Ask: &storagemarket.StorageAsk{
				Miner:         addr2,
				Price:         big.NewInt(0),
				VerifiedPrice: big.NewInt(0),
			},
		},
	}
}

func testGetStorageAsk(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ask := storageAskCase[0]
	dbAsk := fromStorageAsk(&ask)

	tmp := make([]interface{}, 0)
	tmp = append(tmp, dbAsk)

	db, err := getDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(tmp)
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.WithContext(context.Background()).Take(&dbAsk, "miner = ?", dbAsk.Miner.String()))

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	ask2, err := r.StorageAskRepo().GetAsk(context.Background(), ask.Ask.Miner)
	assert.NoError(t, err)
	assert.Equal(t, ask, *ask2)
}

func TestStorageAsk(t *testing.T) {
	r, mock, sqlDB := setup(t)

	t.Run("mysql test GetAsk", wrapper(testGetStorageAsk, r, mock))
	// t.Run("mysql test SetAsk", wrapper(testSetStorageAsk, r, mock))
	// t.Run("mysql test ListAsk", wrapper(testListStorageAsk, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}
