package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

var cidInfoCases []cidInfo

func init() {
	cid1, err := getTestCid()
	if err != nil {
		panic(err)
	}
	cid2, err := getTestCid()
	if err != nil {
		panic(err)
	}
	cidInfoCases = []cidInfo{
		{
			PieceCid:   DBCid(cid1),
			PayloadCid: DBCid(cid1),
			BlockLocation: mysqlBlockLocation{
				RelOffset: 0,
				BlockSize: 0,
			},
		},
		{
			PieceCid:   DBCid(cid2),
			PayloadCid: DBCid(cid2),
			BlockLocation: mysqlBlockLocation{
				RelOffset: 0,
				BlockSize: 0,
			},
		},
	}
}

func TestCidInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	t.Run("mysql test GetCIDInfo", wrapper(testGetCIDInfo, r, mock))
	t.Run("mysql test ListCidInfoKeys", wrapper(testListCidInfoKeys, r, mock))
	t.Run("mysql test AddPieceBlockLocations", wrapper(testAddPieceBlockLocations, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func testGetCIDInfo(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	cidInfoCase := cidInfoCases[0]

	pCidinfo := piecestore.CIDInfo{
		CID: cidInfoCase.PayloadCid.cid(),
		PieceBlockLocations: []piecestore.PieceBlockLocation{
			{BlockLocation: piecestore.BlockLocation(cidInfoCase.BlockLocation),
				PieceCID: cid.Cid(cidInfoCase.PieceCid),
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

func testListCidInfoKeys(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testAddPieceBlockLocations(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

	v1, err := mysqlBlockLocation(blockLocationCase[cid1]).Value()
	assert.NoError(t, err)
	v2, err := mysqlBlockLocation(blockLocationCase[cid2]).Value()
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `cid_infos` (`piece_cid`,`payload_cid`,`block_location`,`created_at`,`updated_at`) VALUES (?,?,?,?,?)")).WithArgs(cid3.String(), cid1.String(), v1, sqlmock.AnyArg(), sqlmock.AnyArg(), cid3.String(), cid2.String(), v2, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(2, 2))
	mock.ExpectCommit()

	err = r.CidInfoRepo().AddPieceBlockLocations(context.Background(), cid3, blockLocationCase)
	assert.NoError(t, err)
}
