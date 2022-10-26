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

func TestSaveChannel(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}
}

func TestGetChannelByAddress(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}

	res, err := r.GetChannelByAddress(ctx, *channelInfoCases[0].Channel)
	assert.NoError(t, err)
	channelInfoCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, channelInfoCases[0], *res)
}

func TestGetChannelByChannelID(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}

	res, err := r.GetChannelByChannelID(ctx, channelInfoCases[0].ChannelID)
	assert.NoError(t, err)
	channelInfoCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, channelInfoCases[0], *res)
}

func TestWithPendingAddFunds(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}

	// refresh the UpdatedAt field of test cases
	for i := 0; i < len(channelInfoCases); i++ {
		res, err := r.GetChannelByAddress(ctx, *channelInfoCases[i].Channel)
		assert.NoError(t, err)
		channelInfoCases[i].UpdatedAt = res.UpdatedAt
	}

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
}

func TestListChannel(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}

	// refresh the UpdatedAt field of test cases
	for i := 0; i < len(channelInfoCases); i++ {
		res, err := r.GetChannelByAddress(ctx, *channelInfoCases[i].Channel)
		assert.NoError(t, err)
		channelInfoCases[i].UpdatedAt = res.UpdatedAt
	}

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
}

func TestCreateChannel(t *testing.T) {
	ctx, r, _ := preparePaychTest(t)

	var paramsCase struct {
		From      address.Address
		To        address.Address
		CreateMsg cid.Cid
		Amt       big.Int
	}
	testutil.Provide(t, &paramsCase)

	_, err := r.CreateChannel(ctx, paramsCase.From, paramsCase.To, paramsCase.CreateMsg, paramsCase.Amt)
	assert.NoError(t, err)
}

func TestGetChannelByMessageCid(t *testing.T) {
	ctx, r, _ := preparePaychTest(t)

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
}

func TestOutboundActiveByFromTo(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	channelInfoCases[0].Direction = types.DirOutbound
	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}

	// refresh the UpdatedAt field of test cases
	for i := 0; i < len(channelInfoCases); i++ {
		res, err := r.GetChannelByAddress(ctx, *channelInfoCases[i].Channel)
		assert.NoError(t, err)
		channelInfoCases[i].UpdatedAt = res.UpdatedAt
	}

	res, err := r.OutboundActiveByFromTo(ctx, channelInfoCases[0].From(), channelInfoCases[0].To())
	assert.NoError(t, err)
	assert.Equal(t, channelInfoCases[0], *res)
}

func TestRemoveChannel(t *testing.T) {
	ctx, r, channelInfoCases := preparePaychTest(t)

	channelInfoCases[0].Direction = types.DirOutbound
	for _, info := range channelInfoCases {
		err := r.SaveChannel(ctx, &info)
		assert.NoError(t, err)
	}

	// refresh the UpdatedAt field of test cases
	for i := 0; i < len(channelInfoCases); i++ {
		res, err := r.GetChannelByAddress(ctx, *channelInfoCases[i].Channel)
		assert.NoError(t, err)
		channelInfoCases[i].UpdatedAt = res.UpdatedAt
	}

	err := r.RemoveChannel(ctx, channelInfoCases[0].ChannelID)
	assert.NoError(t, err)
	_, err = r.GetChannelByAddress(ctx, *channelInfoCases[0].Channel)
	assert.True(t, errors.Is(err, mrepo.ErrNotFound))
}

func TestSaveMessage(t *testing.T) {
	ctx, r, messageInfoCases := preparePaychMsgTest(t)

	for _, info := range messageInfoCases {
		err := r.SaveMessage(ctx, &info)
		assert.NoError(t, err)
	}
}

func TestGetMessage(t *testing.T) {
	ctx, r, messageInfoCases := preparePaychMsgTest(t)

	for _, info := range messageInfoCases {
		err := r.SaveMessage(ctx, &info)
		assert.NoError(t, err)
	}

	res, err := r.GetMessage(ctx, messageInfoCases[0].MsgCid)
	assert.NoError(t, err)
	messageInfoCases[0].UpdatedAt = res.UpdatedAt
	assert.Equal(t, messageInfoCases[0], *res)
}

func TestSaveMessageResult(t *testing.T) {
	ctx, r, messageInfoCases := preparePaychMsgTest(t)

	for _, info := range messageInfoCases {
		err := r.SaveMessage(ctx, &info)
		assert.NoError(t, err)
	}

	err := r.SaveMessageResult(ctx, messageInfoCases[0].MsgCid, errors.New("test error"))
	assert.NoError(t, err)

	res, err := r.GetMessage(ctx, messageInfoCases[0].MsgCid)
	assert.NoError(t, err)

	assert.Equal(t, "test error", res.Err)
}

func preparePaychTest(t *testing.T) (context.Context, mrepo.PaychChannelInfoRepo, []types.ChannelInfo) {
	ctx := context.Background()
	repo := setup(t).PaychChannelInfoRepo()
	channelInfoCases := make([]types.ChannelInfo, 10)
	testutil.Provide(t, &channelInfoCases)
	channelInfoCases[0].Direction = types.DirOutbound
	return ctx, repo, channelInfoCases
}

func preparePaychMsgTest(t *testing.T) (context.Context, mrepo.PaychMsgInfoRepo, []types.MsgInfo) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.PaychMsgInfoRepo()

	messageInfoCases := make([]types.MsgInfo, 10)
	testutil.Provide(t, &messageInfoCases)

	return ctx, r, messageInfoCases
}
