package badger

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	mrepo "github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestPaych(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.PaychChannelInfoRepo()

	channelInfoCases := make([]types.ChannelInfo, 10)
	testutil.Provide(t, &channelInfoCases)
	channelInfoCases[0].Direction = types.DirOutbound

	t.Run("SaveChannel", func(t *testing.T) {
		for _, info := range channelInfoCases {
			err := r.SaveChannel(ctx, &info)
			assert.NoError(t, err)
		}
	})

	t.Run("GetChannelByAddress", func(t *testing.T) {
		res, err := r.GetChannelByAddress(ctx, *channelInfoCases[0].Channel)
		assert.NoError(t, err)
		channelInfoCases[0].UpdatedAt = res.UpdatedAt
		assert.Equal(t, channelInfoCases[0], *res)
	})

	t.Run("GetChannelByChannelID", func(t *testing.T) {
		res, err := r.GetChannelByChannelID(ctx, channelInfoCases[0].ChannelID)
		assert.NoError(t, err)
		channelInfoCases[0].UpdatedAt = res.UpdatedAt
		assert.Equal(t, channelInfoCases[0], *res)
	})

	t.Run("WithPendingAddFunds", func(t *testing.T) {
		expect := make([]types.ChannelInfo, 0)
		for _, info := range channelInfoCases {
			if info.Direction == types.DirOutbound && (info.CreateMsg != nil || info.AddFundsMsg != nil) {
				expect = append(expect, info)
			}
		}

		res, err := r.WithPendingAddFunds(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(expect), len(res))
		for i := 0; i < len(res); i++ {
			assert.Contains(t, expect, *res[i])
		}
	})

	// refresh the UpdatedAt field of test cases
	for i := 0; i < len(channelInfoCases); i++ {
		res, err := r.GetChannelByAddress(ctx, *channelInfoCases[i].Channel)
		assert.NoError(t, err)
		channelInfoCases[i].UpdatedAt = res.UpdatedAt
	}

	t.Run("ListChannel", func(t *testing.T) {
		res, err := r.ListChannel(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(channelInfoCases), len(res))
		addrs := make([]address.Address, 0)
		for _, info := range channelInfoCases {
			addrs = append(addrs, *info.Channel)
		}
		for i := 0; i < len(res); i++ {
			assert.Contains(t, addrs, res[i])
		}
	})

	t.Run("CreateChannel and GetChannelByMessageCid", func(t *testing.T) {
		var paramsCase struct {
			From      address.Address
			To        address.Address
			CreateMsg cid.Cid
			Amt       big.Int
		}

		testutil.Provide(t, &paramsCase)

		_, err := r.CreateChannel(ctx, paramsCase.From, paramsCase.To, paramsCase.CreateMsg, paramsCase.Amt)
		assert.NoError(t, err)

		_, err = r.GetChannelByMessageCid(ctx, paramsCase.CreateMsg)
		assert.NoError(t, err)
	})

	t.Run("OutboundActiveByFromTo", func(t *testing.T) {
		res, err := r.OutboundActiveByFromTo(ctx, channelInfoCases[0].From(), channelInfoCases[0].To())
		assert.NoError(t, err)
		assert.Equal(t, channelInfoCases[0], *res)
	})

	t.Run("RemoveChannel", func(t *testing.T) {
		err := r.RemoveChannel(ctx, channelInfoCases[0].ChannelID)
		assert.NoError(t, err)
		_, err = r.GetChannelByAddress(ctx, *channelInfoCases[0].Channel)
		assert.True(t, errors.Is(err, mrepo.ErrNotFound))
	})
}

func TestMessage(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.PaychMsgInfoRepo()

	messageInfoCases := make([]types.MsgInfo, 10)
	testutil.Provide(t, &messageInfoCases)

	t.Run("SaveMessage", func(t *testing.T) {
		for _, info := range messageInfoCases {
			err := r.SaveMessage(ctx, &info)
			assert.NoError(t, err)
		}
	})

	t.Run("GetMessage", func(t *testing.T) {
		res, err := r.GetMessage(ctx, messageInfoCases[0].MsgCid)
		assert.NoError(t, err)
		messageInfoCases[0].UpdatedAt = res.UpdatedAt
		assert.Equal(t, messageInfoCases[0], *res)
	})

	t.Run("SaveMessageResult", func(t *testing.T) {
		err := r.SaveMessageResult(ctx, messageInfoCases[0].MsgCid, errors.New("test error"))
		assert.NoError(t, err)

		res, err := r.GetMessage(ctx, messageInfoCases[0].MsgCid)
		assert.NoError(t, err)

		assert.Equal(t, "test error", res.Err)
	})
}
