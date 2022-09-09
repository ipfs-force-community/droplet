package mysql

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	market_types "github.com/filecoin-project/venus/venus-shared/types/market"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var channelInfosCase []*market_types.ChannelInfo
var msgInfosCase []*market_types.MsgInfo

func init() {
	addr, err := address.NewIDAddress(10)
	if err != nil {
		panic(err)
	}

	cid, err := getTestCid()
	if err != nil {
		panic(err)
	}

	channelInfosCase = []*market_types.ChannelInfo{
		{
			ChannelID:     uuid.NewString(),
			Channel:       &addr,
			Control:       addr,
			Target:        addr,
			CreateMsg:     &cid,
			AddFundsMsg:   &cid,
			Amount:        big.NewInt(0),
			PendingAmount: big.NewInt(0),
		},
		{
			ChannelID:     uuid.NewString(),
			Channel:       &addr,
			Control:       addr,
			Target:        addr,
			CreateMsg:     &cid,
			AddFundsMsg:   &cid,
			Amount:        big.NewInt(0),
			PendingAmount: big.NewInt(0),
		},
	}

	msgInfosCase = []*market_types.MsgInfo{
		{
			ChannelID: channelInfosCase[0].ChannelID,
			MsgCid:    cid,
		},
		{
			ChannelID: channelInfosCase[0].ChannelID,
			MsgCid:    cid,
		},
	}
}

func TestChannelInfo(t *testing.T) {
	r, mock, sqlDB := setup(t)

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
	channelInfo := channelInfosCase[0]
	dbChannelInfo := fromChannelInfo(channelInfo)
	mock.ExpectBegin()
	// mock.ExpectExec("INSERT INTO `channel_infos`").
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `channel_infos` (`channel_id`,`channel`,`control`,`target`,`direction`,`next_lane`,`amount`,`pending_amount`,`create_msg`,`add_funds_msg`,`settling`,`voucher_info`,`is_deleted`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")).
		WithArgs(dbChannelInfo.ChannelID, dbChannelInfo.Channel, dbChannelInfo.Control, dbChannelInfo.Target, dbChannelInfo.Direction, dbChannelInfo.NextLane, dbChannelInfo.Amount, dbChannelInfo.PendingAmount, dbChannelInfo.CreateMsg, dbChannelInfo.AddFundsMsg, dbChannelInfo.Settling, dbChannelInfo.VoucherInfo, dbChannelInfo.IsDeleted, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.PaychChannelInfoRepo().SaveChannel(context.Background(), channelInfo)
	assert.NoError(t, err)
}

func testGetChannelByAddress(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	channelInfo := channelInfosCase[0]
	dbChannelInfo := fromChannelInfo(channelInfo)

	voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"channel_id", "channel", "control", "target", "direction", "next_lane", "amount", "pending_amount", "create_msg", "add_funds_msg", "settling", "voucher_info", "is_deleted", "created_at", "updated_at"}).AddRow(dbChannelInfo.ChannelID, []byte(dbChannelInfo.Channel.String()), []byte(dbChannelInfo.Control.String()), []byte(dbChannelInfo.Target.String()), dbChannelInfo.Direction, dbChannelInfo.NextLane, dbChannelInfo.Amount, dbChannelInfo.PendingAmount, []byte(dbChannelInfo.CreateMsg.String()), []byte(dbChannelInfo.AddFundsMsg.String()), dbChannelInfo.Settling, voucherInfo, dbChannelInfo.IsDeleted, dbChannelInfo.CreatedAt, dbChannelInfo.UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE channel = ? and is_deleted = 0 LIMIT 1")).
		WithArgs(dbChannelInfo.Channel).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().GetChannelByAddress(context.Background(), *channelInfo.Channel)
	assert.NoError(t, err)
	assert.Equal(t, channelInfo, res)
}

func testGetChannelByChannelID(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	channelInfo := channelInfosCase[0]
	dbChannelInfo := fromChannelInfo(channelInfo)
	voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"channel_id", "channel", "control", "target", "direction", "next_lane", "amount", "pending_amount", "create_msg", "add_funds_msg", "settling", "voucher_info", "is_deleted", "created_at", "updated_at"}).AddRow(dbChannelInfo.ChannelID, []byte(dbChannelInfo.Channel.String()), []byte(dbChannelInfo.Control.String()), []byte(dbChannelInfo.Target.String()), dbChannelInfo.Direction, dbChannelInfo.NextLane, dbChannelInfo.Amount, dbChannelInfo.PendingAmount, []byte(dbChannelInfo.CreateMsg.String()), []byte(dbChannelInfo.AddFundsMsg.String()), dbChannelInfo.Settling, voucherInfo, dbChannelInfo.IsDeleted, dbChannelInfo.CreatedAt, dbChannelInfo.UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE channel_id = ? and is_deleted = 0 LIMIT 1")).WithArgs(dbChannelInfo.ChannelID).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().GetChannelByChannelID(context.Background(), channelInfo.ChannelID)
	assert.NoError(t, err)
	assert.Equal(t, channelInfo, res)
}

func testOutboundActiveByFromTo(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	channelInfo := channelInfosCase[0]
	channelInfo.Direction = market_types.DirOutbound
	channelInfo.Settling = false

	dbChannelInfo := fromChannelInfo(channelInfo)
	voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"channel_id", "channel", "control", "target", "direction", "next_lane", "amount", "pending_amount", "create_msg", "add_funds_msg", "settling", "voucher_info", "is_deleted", "created_at", "updated_at"}).AddRow(dbChannelInfo.ChannelID, []byte(dbChannelInfo.Channel.String()), []byte(dbChannelInfo.Control.String()), []byte(dbChannelInfo.Target.String()), dbChannelInfo.Direction, dbChannelInfo.NextLane, dbChannelInfo.Amount, dbChannelInfo.PendingAmount, []byte(dbChannelInfo.CreateMsg.String()), []byte(dbChannelInfo.AddFundsMsg.String()), dbChannelInfo.Settling, voucherInfo, dbChannelInfo.IsDeleted, dbChannelInfo.CreatedAt, dbChannelInfo.UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE direction = ? and settling = ? and control = ? and target = ? and is_deleted = 0 LIMIT 1")).WithArgs(types.DirOutbound, false, dbChannelInfo.Control.String(), dbChannelInfo.Target.String()).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().OutboundActiveByFromTo(context.Background(), channelInfo.Control, channelInfo.Target)
	assert.NoError(t, err)
	assert.Equal(t, channelInfo, res)
}

func testWithPendingAddFunds(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	dbChannelInfos := make([]*channelInfo, len(channelInfosCase))
	for i, channelInfo := range channelInfosCase {
		tempChannelInfo := channelInfo
		tempChannelInfo.Direction = market_types.DirOutbound

		dbChannelInfos[i] = fromChannelInfo(tempChannelInfo)
	}

	voucherInfos := make([]driver.Value, len(dbChannelInfos))
	for i, dbChannelInfo := range dbChannelInfos {
		voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
		assert.NoError(t, err)
		voucherInfos[i] = voucherInfo
	}

	rows := sqlmock.NewRows([]string{"channel_id", "channel", "control", "target", "direction", "next_lane", "amount", "pending_amount", "create_msg", "add_funds_msg", "settling", "voucher_info", "is_deleted", "created_at", "updated_at"}).AddRow(dbChannelInfos[0].ChannelID, []byte(dbChannelInfos[0].Channel.String()), []byte(dbChannelInfos[0].Control.String()), []byte(dbChannelInfos[0].Target.String()), dbChannelInfos[0].Direction, dbChannelInfos[0].NextLane, dbChannelInfos[0].Amount, dbChannelInfos[0].PendingAmount, []byte(dbChannelInfos[0].CreateMsg.String()), []byte(dbChannelInfos[0].AddFundsMsg.String()), dbChannelInfos[0].Settling, voucherInfos[0], dbChannelInfos[0].IsDeleted, dbChannelInfos[0].CreatedAt, dbChannelInfos[0].UpdatedAt).AddRow(dbChannelInfos[1].ChannelID, []byte(dbChannelInfos[1].Channel.String()), []byte(dbChannelInfos[1].Control.String()), []byte(dbChannelInfos[1].Target.String()), dbChannelInfos[1].Direction, dbChannelInfos[1].NextLane, dbChannelInfos[1].Amount, dbChannelInfos[1].PendingAmount, []byte(dbChannelInfos[1].CreateMsg.String()), []byte(dbChannelInfos[1].AddFundsMsg.String()), dbChannelInfos[1].Settling, voucherInfos[1], dbChannelInfos[1].IsDeleted, dbChannelInfos[1].CreatedAt, dbChannelInfos[1].UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE direction = ? and is_deleted = 0")).WithArgs(types.DirOutbound).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().WithPendingAddFunds(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, channelInfosCase, res)
}

func testListChannel(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	dbChannelInfos := make([]*channelInfo, len(channelInfosCase))
	for i, channelInfo := range channelInfosCase {
		dbChannelInfos[i] = fromChannelInfo(channelInfo)
	}

	voucherInfos := make([]driver.Value, len(dbChannelInfos))
	for i, dbChannelInfo := range dbChannelInfos {
		voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
		assert.NoError(t, err)
		voucherInfos[i] = voucherInfo
	}

	addrs := []address.Address{*channelInfosCase[0].Channel, *channelInfosCase[1].Channel}

	rows := sqlmock.NewRows([]string{"channel_id", "channel", "control", "target", "direction", "next_lane", "amount", "pending_amount", "create_msg", "add_funds_msg", "settling", "voucher_info", "is_deleted", "created_at", "updated_at"}).AddRow(dbChannelInfos[0].ChannelID, []byte(dbChannelInfos[0].Channel.String()), []byte(dbChannelInfos[0].Control.String()), []byte(dbChannelInfos[0].Target.String()), dbChannelInfos[0].Direction, dbChannelInfos[0].NextLane, dbChannelInfos[0].Amount, dbChannelInfos[0].PendingAmount, []byte(dbChannelInfos[0].CreateMsg.String()), []byte(dbChannelInfos[0].AddFundsMsg.String()), dbChannelInfos[0].Settling, voucherInfos[0], dbChannelInfos[0].IsDeleted, dbChannelInfos[0].CreatedAt, dbChannelInfos[0].UpdatedAt).AddRow(dbChannelInfos[1].ChannelID, []byte(dbChannelInfos[1].Channel.String()), []byte(dbChannelInfos[1].Control.String()), []byte(dbChannelInfos[1].Target.String()), dbChannelInfos[1].Direction, dbChannelInfos[1].NextLane, dbChannelInfos[1].Amount, dbChannelInfos[1].PendingAmount, []byte(dbChannelInfos[1].CreateMsg.String()), []byte(dbChannelInfos[1].AddFundsMsg.String()), dbChannelInfos[1].Settling, voucherInfos[1], dbChannelInfos[1].IsDeleted, dbChannelInfos[1].CreatedAt, dbChannelInfos[1].UpdatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE channel != ? and is_deleted = 0")).WithArgs(UndefDBAddress.String()).WillReturnRows(rows)

	res, err := r.PaychChannelInfoRepo().ListChannel(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, addrs, res)
}

func testRemoveChannel(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	channelInfo := channelInfosCase[0]
	dbChannelInfo := fromChannelInfo(channelInfo)
	voucherInfo, err := dbChannelInfo.VoucherInfo.Value()
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `channel_infos` WHERE channel_id = ? and is_deleted = 0 LIMIT 1")).WithArgs(channelInfosCase[0].ChannelID).WillReturnRows(sqlmock.NewRows([]string{"channel_id", "channel", "control", "target", "direction", "next_lane", "amount", "pending_amount", "create_msg", "add_funds_msg", "settling", "voucher_info", "is_deleted", "created_at", "updated_at"}).AddRow(dbChannelInfo.ChannelID, []byte(dbChannelInfo.Channel.String()), []byte(dbChannelInfo.Control.String()), []byte(dbChannelInfo.Target.String()), dbChannelInfo.Direction, dbChannelInfo.NextLane, dbChannelInfo.Amount, dbChannelInfo.PendingAmount, []byte(dbChannelInfo.CreateMsg.String()), []byte(dbChannelInfo.AddFundsMsg.String()), dbChannelInfo.Settling, voucherInfo, dbChannelInfo.IsDeleted, dbChannelInfo.CreatedAt, dbChannelInfo.UpdatedAt))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `channel_infos` SET `is_deleted`=?,`updated_at`=? WHERE channel_id = ?")).WithArgs(1, sqlmock.AnyArg(), channelInfosCase[0].ChannelID).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.PaychChannelInfoRepo().RemoveChannel(context.Background(), channelInfosCase[0].ChannelID)
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
