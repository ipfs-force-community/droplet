package models

import (
	"os"
	"testing"

	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/itf"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/types"
	paychTypes "github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/xerrors"
)

func TestPaych(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		testChannelInfo(t, MysqlDB(t).PaychChannelInfoRepo(), MysqlDB(t).PaychMsgInfoRepo())
		testMsgInfo(t, MysqlDB(t).PaychMsgInfoRepo())
	})

	t.Run("badger", func(t *testing.T) {
		path := "./badger_paych_db"
		db := BadgerDB(t, path)
		defer func() {
			assert.Nil(t, db.Close())
			assert.Nil(t, os.RemoveAll(path))

		}()
		ps := badger.NewPaychStore(db)
		testChannelInfo(t, itf.PaychChannelInfoRepo(ps), itf.PaychMsgInfoRepo(ps))
		testMsgInfo(t, itf.PaychMsgInfoRepo(ps))
	})
}

func testChannelInfo(t *testing.T, channelRepo itf.PaychChannelInfoRepo, msgRepo itf.PaychMsgInfoRepo) {
	msgInfo := &types.MsgInfo{
		ChannelID: uuid.New().String(),
		MsgCid:    randCid(t),
		Received:  false,
		Err:       "",
	}
	assert.Nil(t, msgRepo.SaveMessage(msgInfo))

	addr := randAddress(t)
	msgCid := randCid(t)
	vouchers := []*types.VoucherInfo{
		{
			Voucher: &paychTypes.SignedVoucher{
				ChannelAddr: addr,
				Nonce:       10,
				Amount:      big.NewInt(100),
				Extra: &paychTypes.ModVerifyParams{
					Actor:  addr,
					Method: 1,
					Data:   nil,
				},
			},
			Proof:     nil,
			Submitted: false,
		},
	}
	ci := &types.ChannelInfo{
		ChannelID:     msgInfo.ChannelID,
		Channel:       &addr,
		Control:       randAddress(t),
		Target:        randAddress(t),
		Direction:     types.DirOutbound,
		Vouchers:      vouchers,
		NextLane:      10,
		Amount:        big.NewInt(10),
		PendingAmount: big.NewInt(100),
		CreateMsg:     &msgCid,
		//AddFundsMsg:   &msgCid,
		Settling: false,
	}

	addr2 := randAddress(t)
	msgCid2 := randCid(t)
	ci2 := &types.ChannelInfo{
		ChannelID:     uuid.NewString(),
		Channel:       &addr2,
		Control:       randAddress(t),
		Target:        randAddress(t),
		Direction:     types.DirInbound,
		Vouchers:      nil,
		NextLane:      102,
		Amount:        big.NewInt(102),
		PendingAmount: big.NewInt(1002),
		CreateMsg:     &msgCid,
		AddFundsMsg:   &msgCid2,
		Settling:      true,
	}

	assert.Nil(t, channelRepo.SaveChannel(ci))
	assert.Nil(t, channelRepo.SaveChannel(ci2))

	res, err := channelRepo.GetChannelByChannelID(ci.ChannelID)
	assert.Nil(t, err)
	compareChannelInfo(t, res, ci)
	res2, err := channelRepo.GetChannelByChannelID(ci2.ChannelID)
	assert.Nil(t, err)
	compareChannelInfo(t, res2, ci2)

	res3, err := channelRepo.GetChannelByAddress(*ci.Channel)
	assert.Nil(t, err)
	compareChannelInfo(t, res3, ci)

	res4, err := channelRepo.GetChannelByMessageCid(msgInfo.MsgCid)
	assert.Nil(t, err)
	compareChannelInfo(t, res4, ci)

	from, to := randAddress(t), randAddress(t)
	chMsgCid := randCid(t)
	amt := big.NewInt(101)
	ciRes, err := channelRepo.CreateChannel(from, to, chMsgCid, amt)
	assert.Nil(t, err)
	ciRes2, err := channelRepo.GetChannelByChannelID(ciRes.ChannelID)
	assert.Nil(t, err)
	assert.Equal(t, ciRes.Control, ciRes2.Control)
	assert.Equal(t, ciRes.Target, ciRes2.Target)
	assert.Equal(t, ciRes.CreateMsg, ciRes2.CreateMsg)
	assert.Equal(t, ciRes.PendingAmount, ciRes2.PendingAmount)
	msgInfoRes, err := msgRepo.GetMessage(chMsgCid)
	assert.Nil(t, err)
	assert.Equal(t, msgInfoRes.ChannelID, ciRes.ChannelID)

	addrs, err := channelRepo.ListChannel()
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, len(addrs), 2)

	res5, err := channelRepo.WithPendingAddFunds()
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, len(res5), 1)
	//assert.Equal(t, res5[0].ChannelID, ci.ChannelID)

	res6, err := channelRepo.OutboundActiveByFromTo(ci.Control, ci.Target)
	assert.Nil(t, err)
	assert.Equal(t, res6.ChannelID, ci.ChannelID)
}

func compareChannelInfo(t *testing.T, actual, expected *types.ChannelInfo) {
	assert.Equal(t, expected.ChannelID, actual.ChannelID)
	assert.Equal(t, expected.Channel, actual.Channel)
	assert.Equal(t, expected.Control, actual.Control)
	assert.Equal(t, expected.Target, actual.Target)
	assert.Equal(t, expected.Direction, actual.Direction)
	assert.Equal(t, expected.Vouchers, actual.Vouchers)
	assert.Equal(t, expected.NextLane, actual.NextLane)
	assert.Equal(t, expected.Amount, actual.Amount)
	assert.Equal(t, expected.PendingAmount, actual.PendingAmount)
	assert.Equal(t, expected.CreateMsg, actual.CreateMsg)
	assert.Equal(t, expected.AddFundsMsg, actual.AddFundsMsg)
	assert.Equal(t, expected.Settling, actual.Settling)
}

func testMsgInfo(t *testing.T, msgRepo itf.PaychMsgInfoRepo) {
	info := &types.MsgInfo{
		ChannelID: uuid.New().String(),
		MsgCid:    randCid(t),
		Received:  false,
		Err:       "",
	}

	info2 := &types.MsgInfo{
		ChannelID: uuid.New().String(),
		MsgCid:    randCid(t),
		Received:  true,
		Err:       "err",
	}

	assert.Nil(t, msgRepo.SaveMessage(info))
	assert.Nil(t, msgRepo.SaveMessage(info2))

	res, err := msgRepo.GetMessage(info.MsgCid)
	assert.Nil(t, err)
	compareMsgInfo(t, res, info)
	res2, err := msgRepo.GetMessage(info2.MsgCid)
	assert.Nil(t, err)
	compareMsgInfo(t, res2, info2)

	errMsg := xerrors.Errorf("test err")
	assert.Nil(t, msgRepo.SaveMessageResult(info.MsgCid, errMsg))
	res3, err := msgRepo.GetMessage(info.MsgCid)
	assert.Nil(t, err)
	assert.Equal(t, res3.Err, errMsg.Error())
}

func compareMsgInfo(t *testing.T, actual, expected *types.MsgInfo) {
	assert.Equal(t, expected.ChannelID, actual.ChannelID)
	assert.Equal(t, expected.MsgCid, actual.MsgCid)
	assert.Equal(t, expected.Err, actual.Err)
}
