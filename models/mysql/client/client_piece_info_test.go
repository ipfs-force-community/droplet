package client

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	"github.com/stretchr/testify/assert"
)

func TestSavePieceInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	var pi client.ClientPieceInfo
	testutil.Provide(t, &pi)

	db, err := mysql.GetMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := mysql.GetSQL(db.Save(fromPieceInfo(&pi)))
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.ClientPieceInfoRepo().SavePieceInfo(&pi)
	assert.NoError(t, err)
	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestGetPieceInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	var pi client.ClientPieceInfo
	testutil.Provide(t, &pi)

	dbPI := fromPieceInfo(&pi)

	rows, err := mysql.GetFullRows(dbPI)
	assert.NoError(t, err)

	sql := "SELECT * FROM `piece_infos` WHERE piece_cid = ? LIMIT 1"
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(dbPI.PieceCID.String()).WillReturnRows(rows)

	ret, err := r.ClientPieceInfoRepo().GetPieceInfo(pi.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, pi, *ret)

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestListPieceInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	var pis []*client.ClientPieceInfo
	testutil.Provide(t, &pis)

	var dbPIs []*pieceInfo
	for _, pi := range pis {
		dbPIs = append(dbPIs, fromPieceInfo(pi))
	}

	rows, err := mysql.GetFullRows(dbPIs)
	assert.NoError(t, err)

	sql := "SELECT * FROM `piece_infos`"
	mock.ExpectQuery(regexp.QuoteMeta(sql)).WillReturnRows(rows)

	ret, err := r.ClientPieceInfoRepo().ListPieceInfo()
	assert.NoError(t, err)
	assert.Equal(t, pis, ret)

	assert.NoError(t, closeDB(mock, sqlDB))
}
