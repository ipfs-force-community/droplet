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

var cidInfoCase []cidInfo

func init() {
	cid1, err := getTestCid()
	cid2, err := getTestCid()
	if err != nil {
		panic(err)
	}
	cidInfoCase = []cidInfo{
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
	cidInfo := cidInfoCase[0]

	pCidinfo := piecestore.CIDInfo{
		CID: cidInfo.PayloadCid.cid(),
		PieceBlockLocations: []piecestore.PieceBlockLocation{
			{BlockLocation: piecestore.BlockLocation(cidInfo.BlockLocation),
				PieceCID: cid.Cid(cidInfo.PieceCid),
			},
		}}

	location, err := cidInfo.BlockLocation.Value()
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `cid_infos` WHERE payload_cid = ?")).WithArgs(cidInfo.PayloadCid.String()).WillReturnRows(sqlmock.NewRows([]string{"piece_cid", "payload_cid", "block_location", "created_at", "updated_at"}).AddRow([]byte(cidInfo.PieceCid.String()), []byte(cidInfo.PayloadCid.String()), location, cidInfo.CreatedAt, cidInfo.UpdatedAt))

	res, err := r.CidInfoRepo().GetCIDInfo(context.Background(), cidInfo.PayloadCid.cid())
	assert.NoError(t, err)
	assert.Equal(t, pCidinfo, res)
}

func testListCidInfoKeys(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload_cid FROM `cid_infos`")).WillReturnRows(sqlmock.NewRows([]string{"payload_cid"}).AddRow([]byte(cidInfoCase[0].PayloadCid.String())).AddRow([]byte(cidInfoCase[1].PayloadCid.String())))

	res, err := r.CidInfoRepo().ListCidInfoKeys(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []cid.Cid{cidInfoCase[0].PayloadCid.cid(), cidInfoCase[1].PayloadCid.cid()}, res)
}

func testAddPieceBlockLocations(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	cid1, err := getTestCid()
	cid2, err := getTestCid()
	cid3, err := getTestCid()
	if err != nil {
		panic(err)
	}

	blockLocationCase := map[cid.Cid]piecestore.BlockLocation{
		cid1: piecestore.BlockLocation{
			RelOffset: 0,
			BlockSize: 0,
		},
		cid2: piecestore.BlockLocation{
			RelOffset: 2,
			BlockSize: 0,
		},
	}

	v1, err := mysqlBlockLocation(blockLocationCase[cid1]).Value()
	v2, err := mysqlBlockLocation(blockLocationCase[cid2]).Value()
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `cid_infos` (`piece_cid`,`payload_cid`,`block_location`,`created_at`,`updated_at`) VALUES (?,?,?,?,?)")).WithArgs(cid3.String(), cid1.String(), v1, sqlmock.AnyArg(), sqlmock.AnyArg(), cid3.String(), cid2.String(), v2, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(2, 2))
	mock.ExpectCommit()

	err = r.CidInfoRepo().AddPieceBlockLocations(context.Background(), cid3, blockLocationCase)
	assert.NoError(t, err)
}
