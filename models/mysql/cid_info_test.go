package mysql

import (
	"context"
	"regexp"
	"sort"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

func prepareCIDInfoTest(t *testing.T) (repo.Repo, sqlmock.Sqlmock, []cidInfo, func()) {
	r, mock, sqlDB := setup(t)
	cidInfoCases := make([]cidInfo, 10)
	testutil.Provide(t, &cidInfoCases)
	return r, mock, cidInfoCases, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func TestGetCIDInfo(t *testing.T) {
	r, mock, cidInfoCases, done := prepareCIDInfoTest(t)
	defer done()

	cidInfoCase := cidInfoCases[0]

	pCidinfo := piecestore.CIDInfo{
		CID: cidInfoCase.PayloadCid.cid(),
		PieceBlockLocations: []piecestore.PieceBlockLocation{
			{
				BlockLocation: piecestore.BlockLocation(cidInfoCase.BlockLocation),
				PieceCID:      cid.Cid(cidInfoCase.PieceCid),
			},
		},
	}

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(cidInfoCase)
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Model(&cidInfo{}).Find(&cidInfo{}, "payload_cid = ?", DBCid(cidInfoCase.PayloadCid.cid()).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.CidInfoRepo().GetCIDInfo(context.Background(), cidInfoCase.PayloadCid.cid())
	assert.NoError(t, err)
	assert.Equal(t, pCidinfo, res)
}

func TestListCidInfoKeys(t *testing.T) {
	r, mock, cidInfoCases, done := prepareCIDInfoTest(t)
	defer done()

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"payload_cid"}).AddRow([]byte(cidInfoCases[0].PayloadCid.String())).AddRow([]byte(cidInfoCases[1].PayloadCid.String()))

	sql, vars, err := getSQL(db.Table(cidInfoTableName).Select("payload_cid"))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.CidInfoRepo().ListCidInfoKeys(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []cid.Cid{cidInfoCases[0].PayloadCid.cid(), cidInfoCases[1].PayloadCid.cid()}, res)
}

func TestAddPieceBlockLocations(t *testing.T) {
	r, mock, _, done := prepareCIDInfoTest(t)
	defer done()

	cid1, err := getTestCid()
	assert.NoError(t, err)
	cid2, err := getTestCid()
	assert.NoError(t, err)
	cid3, err := getTestCid()
	assert.NoError(t, err)

	blockLocationCase := map[cid.Cid]piecestore.BlockLocation{
		cid1: {
			RelOffset: 0,
			BlockSize: 0,
		},
		cid2: {
			RelOffset: 2,
			BlockSize: 0,
		},
	}

	// keep the same order with AddPieceBlockLocations
	cids := []cid.Cid{cid1, cid2}
	sort.Slice(cids, func(i, j int) bool {
		return cids[i].String() < cids[j].String()
	})

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Clauses(
		clause.OnConflict{Columns: []clause.Column{{Name: "proposal_cid"}}, UpdateAll: true}).
		Create(toCidInfos(cid3, blockLocationCase)))
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.CidInfoRepo().AddPieceBlockLocations(context.Background(), cid3, blockLocationCase)
	assert.NoError(t, err)
}
