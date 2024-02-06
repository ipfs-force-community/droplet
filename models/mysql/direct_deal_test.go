package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/assert"
)

func TestSaveDirectDeal(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	dbDeal := fromDirectDeal(&deal)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)
	sql, vars, err := getSQL(db.WithContext(ctx).Save(dbDeal))
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.DirectDealRepo().SaveDeal(ctx, &deal)
	assert.Nil(t, err)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetDirectDeal(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	dbDeal := fromDirectDeal(&deal)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `direct_deals` WHERE id = ? LIMIT 1")).WithArgs(dbDeal.ID).WillReturnRows(rows)

	res, err := r.DirectDealRepo().GetDeal(ctx, deal.ID)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetDirectDealByAllocationID(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	dbDeal := fromDirectDeal(&deal)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `direct_deals` WHERE allocation_id = ? and state != ? LIMIT 1")).
		WithArgs(dbDeal.AllocationID, types.DealError).WillReturnRows(rows)

	res, err := r.DirectDealRepo().GetDealByAllocationID(ctx, uint64(deal.AllocationID))
	assert.Nil(t, err)
	assert.NotNil(t, res)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetDirectDealMinerAndState(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	dbDeal := fromDirectDeal(&deal)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `direct_deals` WHERE state = ? and provider = ?")).
		WithArgs(types.DealActive, dbDeal.Provider).WillReturnRows(rows)

	res, err := r.DirectDealRepo().GetDealsByMinerAndState(ctx, deal.Provider, types.DealActive)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestListDirectDeal(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deals []*types.DirectDeal
	testutil.Provide(t, &deals)
	dbDeals := make([]*directDeal, 0, len(deals))
	for _, deal := range deals {
		dbDeals = append(dbDeals, fromDirectDeal(deal))
	}

	firstDeal := deals[0]
	queryParams := types.DirectDealQueryParams{
		Client:   firstDeal.Client,
		Provider: firstDeal.Provider,
		State:    &firstDeal.State,
		Asc:      true,
		Page: types.Page{
			Limit:  10,
			Offset: 0,
		},
	}

	rows, err := getFullRows(dbDeals)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `direct_deals` WHERE state = ? AND provider = ? AND client = ?")).
		WithArgs(firstDeal.State, DBAddress(firstDeal.Provider), DBAddress(firstDeal.Client)).
		WillReturnRows(rows)

	res, err := r.DirectDealRepo().ListDeal(ctx, queryParams)
	assert.Nil(t, err)
	assert.Len(t, res, len(deals))

	assert.NoError(t, closeDB(mock, sqlDB))
}
