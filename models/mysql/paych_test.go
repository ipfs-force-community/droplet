package mysql

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/filecoin-project/go-state-types/big"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

func prepareChannelInfoTest(t *testing.T) (repo.Repo, sqlmock.Sqlmock, []*types.ChannelInfo, func()) {
	r, mock, sqlDB := setup(t)

	channelInfosCases := make([]*types.ChannelInfo, 10)
	testutil.Provide(t, &channelInfosCases)
	for _, ch := range channelInfosCases {
		ch.PendingAvailableAmount = big.NewInt(0)
		ch.AvailableAmount = big.NewInt(0)
	}

	return r, mock, channelInfosCases, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func prepareMegInfoTest(t *testing.T) (repo.Repo, sqlmock.Sqlmock, []*types.MsgInfo, func()) {
	r, mock, sqlDB := setup(t)

	msgInfosCase := make([]*types.MsgInfo, 10)
	testutil.Provide(t, &msgInfosCase)

	return r, mock, msgInfosCase, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func TestSaveChannel(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	channelInfo := channelInfosCases[0]
	dbChannelInfo := fromChannelInfo(channelInfo)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.WithContext(context.Background()).Clauses(clause.OnConflict{UpdateAll: true}).Create(dbChannelInfo))
	assert.NoError(t, err)

	// set updated_at and created_at as any
	vars[len(vars)-1] = sqlmock.AnyArg()
	vars[len(vars)-2] = sqlmock.AnyArg()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.PaychChannelInfoRepo().SaveChannel(context.Background(), channelInfo)
	assert.NoError(t, err)
}

func TestGetChannelByAddress(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	channelInfoCase := channelInfosCases[0]
	dbChannelInfo := fromChannelInfo(channelInfoCase)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbChannelInfo)
	assert.NoError(t, err)

	var nullInfo channelInfo
	sql, vars, err := getSQL(db.Take(&nullInfo, "channel = ? and is_deleted = 0", DBAddress(*channelInfoCase.Channel).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().GetChannelByAddress(context.Background(), *channelInfoCase.Channel)
	assert.NoError(t, err)
	assert.Equal(t, channelInfoCase, res)
}

func TestGetChannelByChannelID(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	channelInfoCase := channelInfosCases[0]
	dbChannelInfo := fromChannelInfo(channelInfoCase)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbChannelInfo)
	assert.NoError(t, err)

	var nullInfo channelInfo
	sql, vars, err := getSQL(db.Take(&nullInfo, "channel_id = ? and is_deleted = 0", channelInfoCase.ChannelID))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().GetChannelByChannelID(context.Background(), channelInfoCase.ChannelID)
	assert.NoError(t, err)
	assert.Equal(t, channelInfoCase, res)
}

func TestOutboundActiveByFromTo(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	channelInfoCase := channelInfosCases[0]
	channelInfoCase.Direction = types.DirOutbound
	channelInfoCase.Settling = false

	dbChannelInfo := fromChannelInfo(channelInfoCase)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbChannelInfo)
	assert.NoError(t, err)

	var nullInfo channelInfo
	sql, vars, err := getSQL(db.Take(&nullInfo, "direction = ? and settling = ? and control = ? and target = ? and is_deleted = 0",
		types.DirOutbound, false, DBAddress(channelInfoCase.Control).String(), DBAddress(channelInfoCase.Target).String()))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().OutboundActiveByFromTo(context.Background(), channelInfoCase.Control, channelInfoCase.Target)
	assert.NoError(t, err)
	assert.Equal(t, channelInfoCase, res)
}

func TestWithPendingAddFunds(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	dbChannelInfos := make([]*channelInfo, len(channelInfosCases))
	for i, channelInfo := range channelInfosCases {
		tempChannelInfo := channelInfo
		tempChannelInfo.Direction = types.DirOutbound
		dbChannelInfos[i] = fromChannelInfo(tempChannelInfo)
	}

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	rows, err := getFullRows(dbChannelInfos)
	assert.NoError(t, err)

	var nullInfo channelInfo
	sql, vars, err := getSQL(db.Find(&nullInfo, "direction = ? and is_deleted = 0", types.DirOutbound))
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().WithPendingAddFunds(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, channelInfosCases, res)
}

func TestListChannel(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	dbChannelInfos := make([]*channelInfo, len(channelInfosCases))
	for i, channelInfo := range channelInfosCases {
		dbChannelInfos[i] = fromChannelInfo(channelInfo)
	}

	voucherInfos := make([]driver.Value, len(dbChannelInfos))
	for i, dbChannelInfo := range dbChannelInfos {
		voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
		assert.NoError(t, err)
		voucherInfos[i] = voucherInfo
	}

	addrs := []address.Address{}
	for _, c := range channelInfosCases {
		addrs = append(addrs, *c.Channel)
	}

	rows, err := getFullRows(dbChannelInfos)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE channel != ? and is_deleted = 0")).WithArgs(UndefDBAddress.String()).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().ListChannel(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, addrs, res)
}

func TestRemoveChannel(t *testing.T) {
	r, mock, channelInfosCases, done := prepareChannelInfoTest(t)
	defer done()

	channelInfo := channelInfosCases[0]
	dbChannelInfo := fromChannelInfo(channelInfo)

	rows, err := getFullRows(dbChannelInfo)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE channel_id = ? and is_deleted = 0 LIMIT 1")).WithArgs(channelInfosCases[0].ChannelID).WillReturnRows(rows)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `channel_infos` SET `is_deleted`=?,`updated_at`=? WHERE channel_id = ?")).WithArgs(1, sqlmock.AnyArg(), channelInfosCases[0].ChannelID).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.PaychChannelInfoRepo().RemoveChannel(context.Background(), channelInfosCases[0].ChannelID)
	assert.NoError(t, err)
}

func TestGetMessage(t *testing.T) {
	r, mock, msgInfosCase, done := prepareMegInfoTest(t)
	defer done()

	msgInfo := msgInfosCase[0]
	dbMsgInfo := fromMsgInfo(msgInfo)

	rows := sqlmock.NewRows([]string{"msg_cid", "channel_id", "received", "err", "created_at", "updated_at"}).AddRow([]byte(dbMsgInfo.MsgCid.String()), dbMsgInfo.ChannelID, dbMsgInfo.Received, dbMsgInfo.Err, dbMsgInfo.CreatedAt, dbMsgInfo.UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `paych_msg_infos` WHERE msg_cid = ? LIMIT 1")).WithArgs(msgInfosCase[0].MsgCid.String()).WillReturnRows(rows)

	res, err := r.PaychMsgInfoRepo().GetMessage(context.Background(), msgInfo.MsgCid)
	assert.NoError(t, err)
	assert.Equal(t, msgInfo, res)
}

func TestSaveMessage(t *testing.T) {
	r, mock, msgInfosCase, done := prepareMegInfoTest(t)
	defer done()

	msgInfo := msgInfosCase[0]
	dbMsgInfo := fromMsgInfo(msgInfo)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.Clauses(clause.OnConflict{UpdateAll: true}).Create(dbMsgInfo))
	assert.NoError(t, err)

	vars[len(vars)-1] = sqlmock.AnyArg()
	vars[len(vars)-2] = sqlmock.AnyArg()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.PaychMsgInfoRepo().SaveMessage(context.Background(), msgInfo)
	assert.NoError(t, err)
}

func TestSaveMessageResult(t *testing.T) {
	r, mock, msgInfosCase, done := prepareMegInfoTest(t)
	defer done()

	msgInfo := msgInfosCase[0]
	dbMsgInfo := fromMsgInfo(msgInfo)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `paych_msg_infos` SET `received`=?,`updated_at`=? WHERE msg_cid = ?")).WithArgs(true, sqlmock.AnyArg(), dbMsgInfo.MsgCid.String()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `paych_msg_infos` SET `err`=?,`received`=?,`updated_at`=? WHERE msg_cid = ?")).WithArgs(errors.New("test").Error(), true, sqlmock.AnyArg(), dbMsgInfo.MsgCid.String()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.PaychMsgInfoRepo().SaveMessageResult(context.Background(), msgInfo.MsgCid, nil)
	assert.NoError(t, err)

	err = r.PaychMsgInfoRepo().SaveMessageResult(context.Background(), msgInfo.MsgCid, errors.New("test"))
	assert.NoError(t, err)
}
