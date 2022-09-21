package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

var storageAskCases []types.SignedStorageAsk

func TestStorageAsk(t *testing.T) {
	addr1 := getTestAddress()
	addr2 := getTestAddress()

	storageAskCases = []types.SignedStorageAsk{
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

	r, mock, sqlDB := setup(t)

	t.Run("mysql test GetAsk", wrapper(testGetStorageAsk, r, mock))
	t.Run("mysql test SetAsk", wrapper(testSetStorageAsk, r, mock))
	t.Run("mysql test ListAsk", wrapper(testListStorageAsk, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func testGetStorageAsk(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ask := storageAskCases[0]
	dbAsk := fromStorageAsk(&ask)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbAsk)
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.WithContext(context.Background()).Take(&dbAsk, "miner = ?", dbAsk.Miner.String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	ask2, err := r.StorageAskRepo().GetAsk(context.Background(), ask.Ask.Miner)
	assert.NoError(t, err)
	assert.Equal(t, ask, *ask2)
}

func testSetStorageAsk(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	ask := storageAskCases[0]
	dbAsk := fromStorageAsk(&ask)

	sql, vars, err := getSQL(db.WithContext(context.Background()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "miner"}},
		UpdateAll: true,
	}).Save(dbAsk))
	assert.NoError(t, err)

	// set updateTime as any
	vars[len(vars)-1] = sqlmock.AnyArg()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = r.StorageAskRepo().SetAsk(context.Background(), &ask)
	assert.NoError(t, err)
}

func testListStorageAsk(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	var dbAsks []*storageAsk
	var expectRes []*types.SignedStorageAsk
	for i := 0; i < len(storageAskCases); i++ {
		dbAsks = append(dbAsks, fromStorageAsk(&storageAskCases[i]))
		expectRes = append(expectRes, &storageAskCases[i])
	}

	rows, err := getFullRows(dbAsks)
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Table("storage_asks").Find(&dbAsks))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	asks, err := r.StorageAskRepo().ListAsk(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectRes, asks)
}
