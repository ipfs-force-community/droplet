package models

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"

	"github.com/filecoin-project/go-state-types/big"
	paychTypes "github.com/filecoin-project/go-state-types/builtin/v8/paych"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPaych(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		testChannelInfo(t, MysqlDB(t).PaychChannelInfoRepo(), MysqlDB(t).PaychMsgInfoRepo())
		testMsgInfo(t, MysqlDB(t).PaychMsgInfoRepo())
	})

	t.Run("badger", func(t *testing.T) {
		db := BadgerDB(t)
		ps := badger.NewPaychRepo(db)
		msgPaych := badger.NewPayMsgRepo(db)
		testChannelInfo(t, ps, msgPaych)
		testMsgInfo(t, msgPaych)
	})
}

func testChannelInfo(t *testing.T, channelRepo repo.PaychChannelInfoRepo, msgRepo repo.PaychMsgInfoRepo) {
	ctx := context.Background()
	msgInfo := &types.MsgInfo{
		ChannelID: uuid.New().String(),
		MsgCid:    randCid(t),
		Received:  false,
		Err:       "",
	}

	assert.Nil(t, msgRepo.SaveMessage(ctx, msgInfo))

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

	ci3 := &types.ChannelInfo{}
	*ci3 = *ci2
	ci3.Channel = nil
	ci3.ChannelID = uuid.NewString()

	assert.Nil(t, channelRepo.SaveChannel(ctx, ci))
	assert.Nil(t, channelRepo.SaveChannel(ctx, ci2))
	assert.Nil(t, channelRepo.SaveChannel(ctx, ci3))

	res, err := channelRepo.GetChannelByChannelID(ctx, ci.ChannelID)
	assert.Nil(t, err)
	assert.Equal(t, res, ci)
	res2, err := channelRepo.GetChannelByChannelID(ctx, ci2.ChannelID)
	assert.Nil(t, err)
	assert.Equal(t, res2, ci2)
	resC3, err := channelRepo.GetChannelByChannelID(ctx, ci3.ChannelID)
	assert.Nil(t, err)
	ci3.Channel = nil
	assert.Equal(t, resC3, ci3)

	res3, err := channelRepo.GetChannelByAddress(ctx, *ci.Channel)
	assert.Nil(t, err)
	assert.Equal(t, res3, ci)

	res4, err := channelRepo.GetChannelByMessageCid(ctx, msgInfo.MsgCid)
	assert.Nil(t, err)
	assert.Equal(t, res4, ci)

	from, to := randAddress(t), randAddress(t)
	chMsgCid := randCid(t)
	amt := big.NewInt(101)
	ciRes, err := channelRepo.CreateChannel(ctx, from, to, chMsgCid, amt)
	assert.Nil(t, err)
	ciRes2, err := channelRepo.GetChannelByChannelID(ctx, ciRes.ChannelID)
	assert.Nil(t, err)
	assert.Equal(t, ciRes.Control, ciRes2.Control)
	assert.Equal(t, ciRes.Target, ciRes2.Target)
	assert.Equal(t, ciRes.CreateMsg, ciRes2.CreateMsg)
	assert.Equal(t, ciRes.PendingAmount, ciRes2.PendingAmount)
	msgInfoRes, err := msgRepo.GetMessage(ctx, chMsgCid)
	assert.Nil(t, err)
	assert.Equal(t, msgInfoRes.ChannelID, ciRes.ChannelID)

	addrs, err := channelRepo.ListChannel(ctx)
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, len(addrs), 2)

	res5, err := channelRepo.WithPendingAddFunds(ctx)
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, len(res5), 1)
	//assert.Equal(t, res5[0].ChannelID, ci.ChannelID)

	res6, err := channelRepo.OutboundActiveByFromTo(ctx, ci.Control, ci.Target)
	assert.Nil(t, err)
	assert.Equal(t, res6.ChannelID, ci.ChannelID)
}

func testMsgInfo(t *testing.T, msgRepo repo.PaychMsgInfoRepo) {
	ctx := context.Background()
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

	assert.Nil(t, msgRepo.SaveMessage(ctx, info))
	assert.Nil(t, msgRepo.SaveMessage(ctx, info2))

	res, err := msgRepo.GetMessage(ctx, info.MsgCid)
	assert.Nil(t, err)
	assert.Equal(t, res, info)
	res2, err := msgRepo.GetMessage(ctx, info2.MsgCid)
	assert.Nil(t, err)
	assert.Equal(t, res2, info2)

	errMsg := fmt.Errorf("test err")
	assert.Nil(t, msgRepo.SaveMessageResult(ctx, info.MsgCid, errMsg))
	res3, err := msgRepo.GetMessage(ctx, info.MsgCid)
	assert.Nil(t, err)
	assert.Equal(t, res3.Err, errMsg.Error())
}
