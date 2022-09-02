package badger

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

const (
	dsKeyChannelInfo = "ChannelInfo"
	dsKeyMsgCid      = "MsgCid"
)

type paychRepo struct {
	ds datastore.Batching
}

func NewPaychRepo(ds PayChanDS) repo.PaychChannelInfoRepo {
	return &paychRepo{ds}
}

func NewPayMsgRepo(ds PayChanDS) repo.PaychMsgInfoRepo {
	return &paychRepo{ds}
}

// CreateChannel creates an outbound channel for the given from / to
func (pr *paychRepo) CreateChannel(ctx context.Context, from address.Address, to address.Address, createMsgCid cid.Cid, amt fbig.Int) (*types.ChannelInfo, error) {
	ci := &types.ChannelInfo{
		Direction:     types.DirOutbound,
		NextLane:      0,
		Control:       from,
		Target:        to,
		CreateMsg:     &createMsgCid,
		PendingAmount: amt,
		ChannelID:     uuid.NewString(),
	}

	// Save the new channel
	err := pr.SaveChannel(ctx, ci)
	if err != nil {
		return nil, err
	}

	// Save a reference to the create message
	err = pr.SaveMessage(ctx, &types.MsgInfo{ChannelID: ci.ChannelID, MsgCid: createMsgCid})
	if err != nil {
		return nil, err
	}

	return ci, err
}

func (pr *paychRepo) GetChannelByAddress(ctx context.Context, ch address.Address) (*types.ChannelInfo, error) {
	return pr.findChan(ctx, func(ci *types.ChannelInfo) bool {
		return ci.Channel != nil && *ci.Channel == ch
	})
}

func (pr *paychRepo) GetChannelByChannelID(ctx context.Context, channelID string) (*types.ChannelInfo, error) {
	return pr.findChan(ctx, func(ci *types.ChannelInfo) bool {
		return ci.ChannelID == channelID
	})
}

func (pr *paychRepo) GetChannelByMessageCid(ctx context.Context, mcid cid.Cid) (*types.ChannelInfo, error) {
	info, err := pr.GetMessage(ctx, mcid)
	if err != nil {
		return nil, err
	}

	ci, err := pr.GetChannelByChannelID(ctx, info.ChannelID)
	if err != nil {
		return nil, err
	}

	return ci, err
}

// OutboundActiveByFromTo looks for outbound channels that have not been
// settled, with the given from / to addresses
func (pr *paychRepo) OutboundActiveByFromTo(ctx context.Context, from address.Address, to address.Address) (*types.ChannelInfo, error) {
	return pr.findChan(ctx, func(ci *types.ChannelInfo) bool {
		if ci.Direction != types.DirOutbound {
			return false
		}
		if ci.Settling {
			return false
		}
		return ci.Control == from && ci.Target == to
	})
}

// ListChannel returns the addresses of all channels that have been created
func (pr *paychRepo) ListChannel(ctx context.Context) ([]address.Address, error) {
	cis, err := pr.findChans(ctx, func(ci *types.ChannelInfo) bool {
		return ci.Channel != nil
	}, 0)
	if err != nil {
		return nil, err
	}

	addrs := make([]address.Address, 0, len(cis))
	for _, ci := range cis {
		addrs = append(addrs, *ci.Channel)
	}

	return addrs, nil
}

// WithPendingAddFunds is used on startup to find channels for which a
// create channel or add funds message has been sent, but shut down
// before the response was received.
func (pr *paychRepo) WithPendingAddFunds(ctx context.Context) ([]*types.ChannelInfo, error) {
	return pr.findChans(ctx, func(ci *types.ChannelInfo) bool {
		if ci.Direction != types.DirOutbound {
			return false
		}
		return ci.CreateMsg != nil || ci.AddFundsMsg != nil
	}, 0)
}

// findChan finds a single channel using the given filter.
// If there isn't a channel that matches the filter, returns repo.ErrNotFound
func (pr *paychRepo) findChan(ctx context.Context, filter func(ci *types.ChannelInfo) bool) (*types.ChannelInfo, error) {
	cis, err := pr.findChans(ctx, filter, 1)
	if err != nil {
		return nil, err
	}

	if len(cis) == 0 {
		return nil, repo.ErrNotFound
	}

	return cis[0], err
}

// findChans loops over all channels, only including those that pass the filter.
// max is the maximum number of channels to return. Set to zero to return unlimited channels.
func (pr *paychRepo) findChans(ctx context.Context, filter func(*types.ChannelInfo) bool, max int) ([]*types.ChannelInfo, error) {
	res, err := pr.ds.Query(ctx, query.Query{Prefix: dsKeyChannelInfo})
	if err != nil {
		return nil, err
	}
	defer res.Close() //nolint:errcheck

	var matches []*types.ChannelInfo

	for {
		res, ok := res.NextSync()
		if !ok {
			break
		}

		if res.Error != nil {
			return nil, err
		}

		var stored types.ChannelInfo
		ci, err := unmarshallChannelInfo(&stored, res.Value)
		if err != nil {
			return nil, err
		}

		if !filter(ci) {
			continue
		}

		matches = append(matches, ci)

		// If we've reached the maximum number of matches, return.
		// Note that if max is zero we return an unlimited number of matches
		// because len(matches) will always be at least 1.
		if len(matches) == max {
			return matches, nil
		}
	}

	return matches, nil
}

// SaveChannel stores the channel info in the datastore
func (pr *paychRepo) SaveChannel(ctx context.Context, ci *types.ChannelInfo) error {
	ci.TimeStamp = makeRefreshedTimeStamp(&ci.TimeStamp)

	key := dskeyForChannel(ci.ChannelID)
	b, err := marshallChannelInfo(ci)
	if err != nil {
		return err
	}

	return pr.ds.Put(ctx, key, b)
}

// RemoveChannel removes the channel with the given channel ID
func (pr *paychRepo) RemoveChannel(ctx context.Context, channelID string) error {
	return pr.ds.Delete(ctx, dskeyForChannel(channelID))
}

// The datastore key used to identify the channel info
func dskeyForChannel(channelID string) datastore.Key {
	return datastore.KeyWithNamespaces([]string{dsKeyChannelInfo, channelID})
}

// TODO: This is a hack to get around not being able to CBOR marshall a nil
// address.Address. It's been fixed in address.Address but we need to wait
// for the change to propagate to specs-actors before we can remove this hack.
var emptyAddr address.Address

func init() {
	addr, err := address.NewActorAddress([]byte("empty"))
	if err != nil {
		panic(err)
	}
	emptyAddr = addr
}

func marshallChannelInfo(ci *types.ChannelInfo) ([]byte, error) {
	// See note above about CBOR marshalling address.Address
	if ci.Channel == nil {
		ci.Channel = &emptyAddr
	}
	return cborutil.Dump(ci)
}

func unmarshallChannelInfo(stored *types.ChannelInfo, value []byte) (*types.ChannelInfo, error) {
	if err := stored.UnmarshalCBOR(bytes.NewReader(value)); err != nil {
		return nil, err
	}

	// See note above about CBOR marshalling address.Address
	if stored.Channel != nil && *stored.Channel == emptyAddr {
		stored.Channel = nil
	}

	return stored, nil
}

// ///// msg info ////////

// GetMessage gets the message info for a given message CID
func (pr *paychRepo) GetMessage(ctx context.Context, mcid cid.Cid) (*types.MsgInfo, error) {
	k := dskeyForMsg(mcid)

	val, err := pr.ds.Get(ctx, k)
	if err != nil {
		return nil, err
	}

	var info types.MsgInfo
	if err := info.UnmarshalCBOR(bytes.NewReader(val)); err != nil {
		return nil, err
	}

	return &info, nil
}

// SaveMessage is called when a message is sent
func (pr *paychRepo) SaveMessage(ctx context.Context, info *types.MsgInfo) error {
	info.TimeStamp = makeRefreshedTimeStamp(&info.TimeStamp)
	k := dskeyForMsg(info.MsgCid)
	b, err := cborutil.Dump(info)
	if err != nil {
		return err
	}

	return pr.ds.Put(ctx, k, b)
}

// SaveMessageResult is called when the result of a message is received
func (pr *paychRepo) SaveMessageResult(ctx context.Context, mcid cid.Cid, msgErr error) error {
	minfo, err := pr.GetMessage(ctx, mcid)
	if err != nil {
		return err
	}

	k := dskeyForMsg(mcid)
	minfo.Received = true
	if msgErr != nil {
		minfo.Err = msgErr.Error()
	}
	minfo.TimeStamp = makeRefreshedTimeStamp(&minfo.TimeStamp)

	b, err := cborutil.Dump(minfo)
	if err != nil {
		return err
	}

	return pr.ds.Put(ctx, k, b)
}

// The datastore key used to identify the message
func dskeyForMsg(mcid cid.Cid) datastore.Key {
	return datastore.KeyWithNamespaces([]string{dsKeyMsgCid, mcid.String()})
}
