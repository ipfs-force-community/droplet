package mysql

import (
	"context"
	"math"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/assert"
)

// fixUint64Fields 修正 Provide 生成的极端 uint64 值，避免 database/sql 报错
func fixUint64Fields(v any) {
	rv := reflect.Indirect(reflect.ValueOf(v)) // 处理指针
	if rv.Kind() != reflect.Struct {
		return
	}

	const max = uint64(math.MaxInt64)

	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() == reflect.Uint64 && f.CanSet() && f.Uint() > max {
			f.SetUint(max) // 或 f.SetUint(0)
		}
	}
}

func TestSaveDirectDeal(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	fixUint64Fields(&deal)

	// Fix a timestamp to avoid drift caused by GORM's automatic updates
	fixedTs := uint64(time.Now().Unix())
	deal.CreatedAt = fixedTs
	deal.UpdatedAt = fixedTs

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

	sql, vars, err = getSQL(db.WithContext(ctx).Where("id = ? and state = ?", deal.ID, dbDeal.State).Save(dbDeal))
	assert.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.DirectDealRepo().SaveDealWithState(ctx, &deal, deal.State)
	assert.Nil(t, err)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetDirectDeal(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	fixUint64Fields(&deal)
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
	fixUint64Fields(&deal)
	dbDeal := fromDirectDeal(&deal)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `direct_deals` WHERE allocation_id = ? LIMIT 1")).
		WithArgs(dbDeal.AllocationID).WillReturnRows(rows)

	res, err := r.DirectDealRepo().GetDealByAllocationID(ctx, deal.AllocationID)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetDirectDealMinerAndState(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	fixUint64Fields(&deal)
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
		fixUint64Fields(deal)
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

func TestGetDirectDealPieceSize(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deals []*types.DirectDeal
	testutil.Provide(t, &deals)
	dbDeals := make([]*directDeal, 0, len(deals))
	for _, deal := range deals {
		fixUint64Fields(deal)
		dbDeals = append(dbDeals, fromDirectDeal(deal))
	}

	deal := deals[0]
	dbDeal := dbDeals[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var nullDeal *directDeal
	sql, vars, err := getSQL(db.Table(directDealTableName).Take(&nullDeal, "piece_cid = ? and state != ?", DBCid(deal.PieceCID).String(), types.DealError))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	playLoadSize, paddedPieceSize, err := r.DirectDealRepo().GetPieceSize(ctx, deal.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, dbDeal.PayloadSize, playLoadSize)
	assert.Equal(t, abi.PaddedPieceSize(dbDeal.PieceSize), paddedPieceSize)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetDirectDealPieceInfo(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deals []*types.DirectDeal
	testutil.Provide(t, &deals)
	dbDeals := make([]*directDeal, 0, len(deals))
	for _, deal := range deals {
		fixUint64Fields(deal)
		dbDeals = append(dbDeals, fromDirectDeal(deal))
	}

	deal := deals[0]
	dbDeal := dbDeals[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var nullDeal *directDeal
	sql, vars, err := getSQL(db.Table(directDealTableName).Find(&nullDeal, "piece_cid = ?", DBCid(deal.PieceCID).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	pInfo := &piecestore.PieceInfo{
		PieceCID: deal.PieceCID,
		Deals: []piecestore.DealInfo{
			{
				Offset:   deal.Offset,
				Length:   deal.PieceSize,
				SectorID: deal.SectorID,
			},
		},
	}

	res, err := r.DirectDealRepo().GetPieceInfo(ctx, deal.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, pInfo, res)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestCountDDODealByMiner(t *testing.T) {
	ctx := context.Background()
	r, mock, sqlDB := setup(t)

	var deal types.DirectDeal
	testutil.Provide(t, &deal)
	fixUint64Fields(&deal)
	deal.State = types.DealActive

	count := int64(1)
	row := sqlmock.NewRows([]string{"count"})
	row.AddRow(count)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `direct_deals` WHERE provider = ? and state = ?")).
		WithArgs(DBAddress(deal.Provider), types.DealActive).WillReturnRows(row)
	count, err := r.DirectDealRepo().CountDealByMiner(ctx, deal.Provider, types.DealActive)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	assert.NoError(t, closeDB(mock, sqlDB))
}