package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/venus/venus-shared/types"
	market_types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/sophon-messager/models/mtypes"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

func prepareRetrievalAskTest(t *testing.T) (repo.Repo, sqlmock.Sqlmock, *market_types.RetrievalAsk, func()) {
	r, mock, sqlDB := setup(t)

	addr := getTestAddress()

	retrievalAskCase := &market_types.RetrievalAsk{
		Miner:                   addr,
		PricePerByte:            types.NewInt(1),
		PaymentInterval:         2,
		PaymentIntervalIncrease: 3,
		UnsealPrice:             types.NewInt(4),
	}

	return r, mock, retrievalAskCase, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func TestRetrievalGetAsk(t *testing.T) {
	r, mock, retrievalAskCase, done := prepareRetrievalAskTest(t)
	defer done()

	ctx := context.Background()

	rows := mock.NewRows([]string{"address", "price_per_byte", "payment_interval", "payment_interval_increase", "unseal_price"})
	rows.AddRow(DBAddress(retrievalAskCase.Miner), mtypes.SafeFromGo(retrievalAskCase.PricePerByte.Int), retrievalAskCase.PaymentInterval, retrievalAskCase.PaymentIntervalIncrease, mtypes.SafeFromGo(retrievalAskCase.UnsealPrice.Int))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_asks` WHERE address = ? LIMIT 1")).WithArgs(DBAddress(retrievalAskCase.Miner).String()).WillReturnRows(rows)
	result, err := r.RetrievalAskRepo().GetAsk(ctx, retrievalAskCase.Miner)
	assert.Nil(t, err)
	assert.Equal(t, retrievalAskCase, result)
}

func TestSetRetrievalAsk(t *testing.T) {
	r, mock, retrievalAskCase, done := prepareRetrievalAskTest(t)
	defer done()

	ctx := context.Background()

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "address"}}, UpdateAll: true}).
		Create(fromRetrievalAsk(retrievalAskCase)))
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.RetrievalAskRepo().SetAsk(ctx, retrievalAskCase)
	assert.Nil(t, err)
}

func TestListRetrievalAsk(t *testing.T) {
	r, mock, retrievalAskCase, done := prepareRetrievalAskTest(t)
	defer done()

	ctx := context.Background()

	rows := mock.NewRows([]string{"address", "price_per_byte", "unseal_price", "payment_interval", "payment_interval_increase", "created_at", "updated_at"})
	rows.AddRow([]byte(DBAddress(retrievalAskCase.Miner).String()), mtypes.SafeFromGo(retrievalAskCase.PricePerByte.Int), mtypes.SafeFromGo(retrievalAskCase.UnsealPrice.Int), retrievalAskCase.PaymentInterval, retrievalAskCase.PaymentIntervalIncrease, retrievalAskCase.CreatedAt, retrievalAskCase.UpdatedAt)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_asks`")).WillReturnRows(rows)
	result, err := r.RetrievalAskRepo().ListAsk(ctx)
	assert.Nil(t, err)
	assert.Equal(t, []*market_types.RetrievalAsk{retrievalAskCase}, result)
}
