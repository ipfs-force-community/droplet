package mysql

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-address"
)

func TestSaveDeal(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Clauses(
		clause.OnConflict{Columns: []clause.Column{{Name: "proposal_cid"}}, UpdateAll: true}).
		Create(dbStorageDealCases[0]))
	assert.NoError(t, err)

	vars[42] = sqlmock.AnyArg()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.StorageDealRepo().SaveDeal(context.Background(), storageDealCases[0])
	assert.NoError(t, err)
}

func TestGetDeal(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	pageSize := 10
	pageIndex := 1

	var md []storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).
		Find(&md, "cdp_provider = ?", dbStorageDealCases[0].ClientDealProposal.Provider.String()).
		Offset(pageIndex * pageSize).Limit(pageSize))
	assert.NoError(t, err)

	rows, err := getFullRows(dbStorageDealCases[0])
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().GetDeals(context.Background(), storageDealCases[0].Proposal.Provider, pageIndex, pageSize)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, storageDealCases[0], res[0])
}

func TestGetDealsByPieceCidAndStatus(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var md []storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).
		Find(&md, "cdp_piece_cid = ? AND state in ?", dbDeal.PieceCID.String(), []storagemarket.StorageDealStatus{storagemarket.StorageDealActive}))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().GetDealsByPieceCidAndStatus(context.Background(), deal.Proposal.PieceCID, storagemarket.StorageDealActive)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, deal, res[0])
}

func TestGetDealsByDataCidAndDealStatus(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var md []storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Where("ref_root=?", deal.Ref.Root.String()).Where("cdp_provider=?", DBAddress(deal.Proposal.Provider).String()).Where("piece_status in ?", []types.PieceStatus{deal.PieceStatus}).Find(&md))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().GetDealsByDataCidAndDealStatus(context.Background(), deal.Proposal.Provider, deal.Ref.Root, []types.PieceStatus{deal.PieceStatus})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, deal, res[0])
}

func TestGetDealByAddrAndStatus(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var md []storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Where("cdp_provider=?", DBAddress(deal.Proposal.Provider).String()).Where("state in ?", []storagemarket.StorageDealStatus{storagemarket.StorageDealActive}).Find(&md))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().GetDealByAddrAndStatus(context.Background(), deal.Proposal.Provider, storagemarket.StorageDealActive)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, deal, res[0])
}

func TestGetGetDealByDealID(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var nullDeal *storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Take(&nullDeal, "cdp_provider = ? and deal_id = ?", DBAddress(deal.Proposal.Provider).String(), deal.DealID))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().GetDealByDealID(context.Background(), deal.Proposal.Provider, deal.DealID)
	assert.NoError(t, err)
	assert.Equal(t, deal, res)
}

func TestGetDealsByPieceStatusAndDealStatus(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	t.Run("with deal status", func(t *testing.T) {
		rows, err := getFullRows(dbDeal)
		assert.NoError(t, err)
		var md []storageDeal
		sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Where("piece_status = ?", deal.PieceStatus).Where("state in ?", []storagemarket.StorageDealStatus{dbDeal.State}).Where("cdp_provider=?", DBAddress(deal.Proposal.Provider).String()).Find(&md))
		assert.NoError(t, err)
		assert.NotEqual(t, "", sql)

		mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

		res, err := r.StorageDealRepo().GetDealsByPieceStatusAndDealStatus(context.Background(), deal.Proposal.Provider, deal.PieceStatus, deal.State)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, deal, res[0])
	})

	t.Run("without deal status", func(t *testing.T) {
		rows, err := getFullRows(dbDeal)
		assert.NoError(t, err)
		var md []storageDeal
		sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Where("piece_status = ?", deal.PieceStatus).Where("cdp_provider=?", DBAddress(deal.Proposal.Provider).String()).Find(&md))
		assert.NoError(t, err)
		assert.NotEqual(t, "", sql)

		mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

		res, err := r.StorageDealRepo().GetDealsByPieceStatusAndDealStatus(context.Background(), deal.Proposal.Provider, deal.PieceStatus)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, deal, res[0])
	})
}

func TestUpdateDealStatus(t *testing.T) {
	r, mock, _, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]

	targetDealStatus := storagemarket.StorageDealAwaitingPreCommit
	targetPieceStatus := types.Assigned

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	updateColumns := map[string]interface{}{"state": targetDealStatus, "piece_status": targetPieceStatus, "updated_at": sqlmock.AnyArg()}

	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Where("proposal_cid = ?", DBCid(deal.ProposalCid).String()).Updates(updateColumns))
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.StorageDealRepo().UpdateDealStatus(context.Background(), deal.ProposalCid, targetDealStatus, targetPieceStatus)
	assert.NoError(t, err)
}

func TestListDeal(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbStorageDealCases)
	assert.NoError(t, err)

	var storageDeals []storageDeal
	ctx := context.Background()
	caseCount := len(storageDealCases)
	defPage := types.Page{Limit: caseCount}
	newQuery := func() *gorm.DB {
		return db.Table((&storageDeal{}).TableName())
	}

	// empty params
	sql, _, err := getSQL(newQuery().Find(&storageDeals))
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WillReturnRows(rows)
	res, err := r.StorageDealRepo().ListDeal(ctx, &types.StorageDealQueryParams{})
	assert.NoError(t, err)
	assert.Len(t, res, 2)

	var vars []driver.Value
	// test page
	rows, err = getFullRows(dbStorageDealCases[1:])
	assert.NoError(t, err)
	sql, vars, err = getSQL(newQuery().Offset(1).Limit(5).Find(&storageDeals))
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)
	res, err = r.StorageDealRepo().ListDeal(ctx, &types.StorageDealQueryParams{Page: types.Page{Offset: 1, Limit: 5}})
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, storageDealCases[1], res[0])

	// test miner
	rows, err = getFullRows(dbStorageDealCases[0])
	assert.NoError(t, err)
	sql, vars, err = getSQL(newQuery().Where("cdp_provider = ?", dbStorageDealCases[0].Provider.String()).Limit(caseCount).Find(&storageDeals))
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)
	res, err = r.StorageDealRepo().ListDeal(ctx, &types.StorageDealQueryParams{Page: defPage, Miner: address.Address(dbStorageDealCases[0].Provider)})
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// test client
	client := dbStorageDealCases[0].Client
	rows, err = getFullRows(dbStorageDealCases[0])
	assert.NoError(t, err)
	sql, vars, err = getSQL(newQuery().Where("client = ?", client).Limit(caseCount).Find(&storageDeals))
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)
	res, err = r.StorageDealRepo().ListDeal(ctx, &types.StorageDealQueryParams{Page: defPage, Client: client})
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// test state
	storageDealActive := storagemarket.StorageDealActive
	rows, err = getFullRows(dbStorageDealCases)
	assert.NoError(t, err)
	sql, vars, err = getSQL(newQuery().Where("state = ?", storageDealActive).Limit(caseCount).Find(&storageDeals))
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)
	res, err = r.StorageDealRepo().ListDeal(ctx, &types.StorageDealQueryParams{Page: defPage, State: &storageDealActive})
	assert.NoError(t, err)
	assert.Len(t, res, 2)

	// test state
	states := []storagemarket.StorageDealStatus{storagemarket.StorageDealFailing,
		storagemarket.StorageDealExpired, storagemarket.StorageDealError, storagemarket.StorageDealSlashed}
	rows, err = getFullRows(dbStorageDealCases)
	assert.NoError(t, err)
	sql, vars, err = getSQL(newQuery().Where("state not in ?", states).Limit(caseCount).Find(&storageDeals))
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)
	res, err = r.StorageDealRepo().ListDeal(ctx, &types.StorageDealQueryParams{Page: defPage, DiscardFailedDeal: true})
	assert.NoError(t, err)
	assert.Len(t, res, 2)
}

func TestListDealByAddr(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]
	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var nullDeals []*storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Find(&nullDeals, "cdp_provider = ?", DBAddress(deal.Proposal.Provider).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().ListDealByAddr(context.Background(), deal.Proposal.Provider)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, deal, res[0])
}

func TestGetPieceInfo(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var nullDeal *storageDeal
	sql, vars, err := getSQL(db.Table(storageDealTableName).Find(&nullDeal, "cdp_piece_cid = ?", DBCid(deal.Proposal.PieceCID).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	pInfo := &piecestore.PieceInfo{
		PieceCID: deal.Proposal.PieceCID,
		Deals: []piecestore.DealInfo{
			{
				DealID:   deal.DealID,
				Offset:   deal.Offset,
				Length:   deal.Proposal.PieceSize,
				SectorID: deal.SectorNumber,
			},
		},
	}

	res, err := r.StorageDealRepo().GetPieceInfo(context.Background(), deal.Proposal.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, pInfo, res)
}

func TestListPieceInfoKeys(t *testing.T) {
	r, mock, dbStorageDealCases, _, done := prepareStorageDealRepoTest(t)
	defer done()

	dbDeal := dbStorageDealCases[0]
	cids, err := cid.Decode(dbDeal.PieceCID.String())
	assert.NoError(t, err)

	pCidV, err := dbDeal.PieceCID.Value()
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"cdp_piece_cid"}).AddRow(pCidV)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Select("cdp_piece_cid"))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().ListPieceInfoKeys(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, cids, res[0])
}

func TestGetPieceSize(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	states := []storagemarket.StorageDealStatus{storagemarket.StorageDealFailing, storagemarket.StorageDealRejecting, storagemarket.StorageDealError}
	var nullDeal *storageDeal
	sql, vars, err := getSQL(db.Table(storageDealTableName).Take(&nullDeal, "cdp_piece_cid = ? and state not in ?", DBCid(deal.Proposal.PieceCID).String(), states))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	playLoadSize, paddedPieceSize, err := r.StorageDealRepo().GetPieceSize(context.Background(), deal.Proposal.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, dbDeal.PayloadSize, playLoadSize)
	assert.Equal(t, abi.PaddedPieceSize(dbDeal.PieceSize), paddedPieceSize)
}

func Test_storageDealRepo_GroupDealsByStatus(t *testing.T) {
	r, mock, _, _, done := prepareStorageDealRepoTest(t)
	defer done()

	ctx := context.Background()

	t.Run("correct", func(t *testing.T) {
		expectResult := map[storagemarket.StorageDealStatus]int64{
			storagemarket.StorageDealActive: 1,
		}
		rows := mock.NewRows([]string{"state", "count"})
		for status, count := range expectResult {
			rows.AddRow(status, count)
		}

		addr := getTestAddress()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT state, count(1) as count FROM `storage_deals` WHERE cdp_provider = ? GROUP BY `state`")).WithArgs(DBAddress(addr).String()).WillReturnRows(rows)
		result, err := r.StorageDealRepo().GroupStorageDealNumberByStatus(ctx, addr)
		assert.Nil(t, err)
		assert.Equal(t, expectResult, result)
	})

	t.Run("undefined address", func(t *testing.T) {
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

func prepareStorageDealRepoTest(t *testing.T) (repo.Repo, sqlmock.Sqlmock, []*storageDeal, []*types.MinerDeal, func()) {
	var dbStorageDealCases []*storageDeal
	var storageDealCases []*types.MinerDeal

	cid1, err := getTestCid()
	assert.NoError(t, err)
	cid2, err := getTestCid()
	assert.NoError(t, err)

	peer1, err := getTestPeerId()
	assert.NoError(t, err)
	peer2, err := getTestPeerId()
	assert.NoError(t, err)

	temp := []*types.MinerDeal{
		{
			ProposalCid: cid1,
			Miner:       peer1,
			Client:      peer1,
			ClientDealProposal: vTypes.ClientDealProposal{
				Proposal: vTypes.DealProposal{
					Provider: getTestAddress(),
					PieceCID: cid1,
				},
			},
			State: storagemarket.StorageDealActive,
			TimeStamp: types.TimeStamp{
				CreatedAt: uint64(time.Now().Unix()),
				UpdatedAt: uint64(time.Now().Unix()),
			},
			Ref: &storagemarket.DataRef{
				Root: cid1,
			},
		},
		{
			ProposalCid: cid2,
			Miner:       peer2,
			Client:      peer2,
			ClientDealProposal: vTypes.ClientDealProposal{
				Proposal: vTypes.DealProposal{
					Provider: getTestAddress(),
					PieceCID: cid2,
				},
			},
			State: storagemarket.StorageDealActive,
			TimeStamp: types.TimeStamp{
				CreatedAt: uint64(time.Now().Unix()),
				UpdatedAt: uint64(time.Now().Unix()),
			},
			Ref: &storagemarket.DataRef{
				Root: cid2,
			},
		},
	}

	storageDealCases = make([]*types.MinerDeal, 0)
	dbStorageDealCases = make([]*storageDeal, 0)
	for _, v := range temp {
		dbDeal := fromStorageDeal(v)
		deal, err := toStorageDeal(dbDeal)
		assert.NoError(t, err)
		dbStorageDealCases = append(dbStorageDealCases, dbDeal)
		storageDealCases = append(storageDealCases, deal)
	}

	r, mock, sqlDB := setup(t)

	return r, mock, dbStorageDealCases, storageDealCases, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func TestGetDealByUUID(t *testing.T) {
	r, mock, dbStorageDealCases, storageDealCases, done := prepareStorageDealRepoTest(t)
	defer done()

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	deal := dbStorageDealCases[0]

	var md storageDeal
	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Take(&md, "id = ?", deal.ID))
	assert.NoError(t, err)

	rows, err := getFullRows(deal)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().GetDealByUUID(context.Background(), storageDealCases[0].ID)
	assert.NoError(t, err)
	assert.Equal(t, storageDealCases[0], res)
}
