package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin/v8/market"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm/clause"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-address"
)

var dbStorageDealCases []*storageDeal
var storageDealCases []*types.MinerDeal

func testSaveDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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
			ClientDealProposal: market.ClientDealProposal{
				Proposal: market.DealProposal{
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
			ClientDealProposal: market.ClientDealProposal{
				Proposal: market.DealProposal{
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

func testGetDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetDealsByPieceCidAndStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetDealsByDataCidAndDealStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetDealByAddrAndStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetGetDealByDealID(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetDealsByPieceStatusAndDealStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testUpdateDealStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testListDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbStorageDealCases)
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Table((&storageDeal{}).TableName()).Find(&[]storageDeal{}))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.StorageDealRepo().ListDeal(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(storageDealCases), len(res))
	for i := 0; i < len(res); i++ {
		assert.Equal(t, storageDealCases[i], res[i])
	}
}

func testListDealByAddr(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetPieceInfo(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testListPieceInfoKeys(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetPieceSize(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	deal := storageDealCases[0]
	dbDeal := dbStorageDealCases[0]

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbDeal)
	assert.NoError(t, err)

	var nullDeal *storageDeal
	sql, vars, err := getSQL(db.Table(storageDealTableName).Take(&nullDeal, "cdp_piece_cid = ? ", DBCid(deal.Proposal.PieceCID).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	playLoadSize, paddedPieceSize, err := r.StorageDealRepo().GetPieceSize(context.Background(), deal.Proposal.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, dbDeal.PayloadSize, playLoadSize)
	assert.Equal(t, abi.PaddedPieceSize(dbDeal.PieceSize), paddedPieceSize)

}

func test_storageDealRepo_GroupDealsByStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func TestStorageDealRepo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	t.Run("mysql test SaveDeal", wrapper(testSaveDeal, r, mock))
	t.Run("mysql test GetDeal", wrapper(testGetDeal, r, mock))
	t.Run("mysql test GetDealsByPieceCidAndStatus", wrapper(testGetDealsByPieceCidAndStatus, r, mock))
	t.Run("mysql test GetDealsByDataCidAndDealStatus", wrapper(testGetDealsByDataCidAndDealStatus, r, mock))

	t.Run("mysql test GetDealByAddrAndStatus", wrapper(testGetDealByAddrAndStatus, r, mock))
	t.Run("mysql test GetDealByDealID", wrapper(testGetGetDealByDealID, r, mock))
	t.Run("mysql test GetDealsByPieceStatus", wrapper(testGetDealsByPieceStatusAndDealStatus, r, mock))

	t.Run("mysql test UpdateDealStatus", wrapper(testUpdateDealStatus, r, mock))
	t.Run("mysql test ListDeal", wrapper(testListDeal, r, mock))
	t.Run("mysql test ListDealByAddr", wrapper(testListDealByAddr, r, mock))
	t.Run("mysql test GetPieceInfo", wrapper(testGetPieceInfo, r, mock))
	t.Run("mysql test ListPieceInfoKeys", wrapper(testListPieceInfoKeys, r, mock))
	t.Run("mysql test GetPieceSize", wrapper(testGetPieceSize, r, mock))
	t.Run("mysql test GroupStorageDealNumberByStatus", wrapper(test_storageDealRepo_GroupDealsByStatus, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}
