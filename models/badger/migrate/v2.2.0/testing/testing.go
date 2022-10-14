package testing

import (
	"context"
	"github.com/libp2p/go-libp2p-core/peer"
	"testing"

	"github.com/filecoin-project/venus-market/v2/models/badger/migrate"

	"github.com/ipfs/go-cid"

	v220 "github.com/filecoin-project/venus-market/v2/models/badger/migrate/v2.2.0"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	cbg "github.com/whyrusleeping/cbor-gen"

	cbor "github.com/filecoin-project/go-cbor-util"
)

func dsPutObj(ctx context.Context, t *testing.T, v migrate.DsKeyAble, ds datastore.Batching) {
	data, err := cbor.Dump(v)
	assert.NoError(t, err)
	assert.NoError(t, ds.Put(ctx, v.KeyWithNamespace(), data))
}

func WriteTestcasesToDS(ctx context.Context, t *testing.T, ds datastore.Batching, count int) (payChMsgCIDs []cid.Cid) {
	var peerIDProvider = func(t *testing.T) peer.ID {
		peerId, err := peer.Decode("12D3KooWMjDC9AtFegcGJPJNvwV5fdiehTmx7awvUTXbktqboKbi")
		assert.NoError(t, err)
		return peerId
	}

	{
		for i := 0; i < count; i++ {
			var stat v220.FundedAddressState
			testutil.Provide(t, &stat)
			dsPutObj(ctx, t, &stat, ds)
		}
	}

	{
		for i := 0; i < count; i++ {
			var deal v220.MinerDeal
			testutil.Provide(t, &deal, peerIDProvider)
			dsPutObj(ctx, t, &deal, ds)
		}
	}

	{
		for i := 0; i < count; i++ {
			var info v220.MsgInfo
			testutil.Provide(t, &info)
			dsPutObj(ctx, t, &info, ds)
			payChMsgCIDs = append(payChMsgCIDs, info.MsgCid)
		}
	}

	{
		for i := 0; i < count; i++ {
			var info v220.ChannelInfo
			testutil.Provide(t, &info)
			dsPutObj(ctx, t, &info, ds)
		}
	}

	{
		for i := 0; i < count; i++ {
			var ask v220.SignedStorageAsk
			testutil.Provide(t, &ask.Ask)
			testutil.Provide(t, &ask.Signature)
			dsPutObj(ctx, t, &ask, ds)
		}
	}

	{
		for i := 0; i < count; i++ {
			var ask v220.RetrievalAsk
			testutil.Provide(t, &ask)
			dsPutObj(ctx, t, &ask, ds)
		}
	}

	{
		for i := 0; i < count; i++ {
			var cidInfo v220.CIDInfo
			testutil.Provide(t, &cidInfo.CID)
			testutil.Provide(t, &cidInfo.PieceBlockLocations, testutil.WithSliceLen(2))
			dsPutObj(ctx, t, &cidInfo, ds)
		}
	}

	{
		for i := 0; i < count; i++ {
			var stat v220.ProviderDealState
			testutil.Provide(t, &stat, peerIDProvider,
				func(t *testing.T) *cbg.Deferred { return nil })
			dsPutObj(ctx, t, &stat, ds)
		}
	}
	return
}
