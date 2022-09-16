package mysql

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

var channelInfosCases []*types.ChannelInfo
var msgInfosCase []*types.MsgInfo

func TestChannelInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	channelInfosCases = make([]*types.ChannelInfo, 10)
	testutil.Provide(t, &channelInfosCases)

	msgInfosCase = make([]*types.MsgInfo, 10)
	testutil.Provide(t, &msgInfosCase)

	t.Run("mysql test SaveChannel", wrapper(testSaveChannel, r, mock))
	t.Run("mysql test GetChannelByAddress", wrapper(testGetChannelByAddress, r, mock))
	t.Run("mysql test GetChannelByChannelID", wrapper(testGetChannelByChannelID, r, mock))
	t.Run("mysql test OutboundActiveByFromTo", wrapper(testOutboundActiveByFromTo, r, mock))
	t.Run("mysql test WithPendingAddFunds", wrapper(testWithPendingAddFunds, r, mock))
	t.Run("mysql test ListChannel", wrapper(testListChannel, r, mock))
	t.Run("mysql test RemoveChannel", wrapper(testRemoveChannel, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func TestMegInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

	t.Run("mysql test GetMessage", wrapper(testGetMessage, r, mock))
	t.Run("mysql test SaveMessage", wrapper(testSaveMessage, r, mock))
	t.Run("mysql test SaveMessageResult", wrapper(testSaveMessageResult, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func testSaveChannel(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	channelInfo := channelInfosCases[0]
	dbChannelInfo := fromChannelInfo(channelInfo)

	db, err := getMysqlDryrunDB()
	assert.NoError(t, err)

	sql, vars, err := getSQL(db.WithContext(context.Background()).Clauses(clause.OnConflict{UpdateAll: true}).Create(dbChannelInfo))
	assert.NoError(t, err)

	// set updated_at field as any
	vars[len(vars)-1] = sqlmock.AnyArg()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.PaychChannelInfoRepo().SaveChannel(context.Background(), channelInfo)
	assert.NoError(t, err)
}

func testGetChannelByAddress(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetChannelByChannelID(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testOutboundActiveByFromTo(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testWithPendingAddFunds(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testListChannel(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testRemoveChannel(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testGetMessage(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	msgInfo := msgInfosCase[0]
	dbMsgInfo := fromMsgInfo(msgInfo)

	rows := sqlmock.NewRows([]string{"msg_cid", "channel_id", "received", "err", "created_at", "updated_at"}).AddRow([]byte(dbMsgInfo.MsgCid.String()), dbMsgInfo.ChannelID, dbMsgInfo.Received, dbMsgInfo.Err, dbMsgInfo.CreatedAt, dbMsgInfo.UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `paych_msg_infos` WHERE msg_cid = ? LIMIT 1")).WithArgs(msgInfosCase[0].MsgCid.String()).WillReturnRows(rows)

	res, err := r.PaychMsgInfoRepo().GetMessage(context.Background(), msgInfo.MsgCid)
	assert.NoError(t, err)
	assert.Equal(t, msgInfo, res)
}

func testSaveMessage(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	msgInfo := msgInfosCase[0]
	dbMsgInfo := fromMsgInfo(msgInfo)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `paych_msg_infos` (`channel_id`,`msg_cid`,`received`,`err`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?)")).WithArgs(dbMsgInfo.ChannelID, dbMsgInfo.MsgCid.String(), dbMsgInfo.Received, dbMsgInfo.Err, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.PaychMsgInfoRepo().SaveMessage(context.Background(), msgInfo)
	assert.NoError(t, err)
}

func testSaveMessageResult(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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
