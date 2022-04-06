package dagstore

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/url"
	"os"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	market0 "github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/venus-market/config"
	mock_dagstore2 "github.com/filecoin-project/venus-market/dagstore/mocks"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/filecoin-project/venus-market/utils"
	builtinMarket "github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipld/go-car/v2"
	carindex "github.com/ipld/go-car/v2/index"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-cid"
	blocksutil "github.com/ipfs/go-ipfs-blocksutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/dagstore/mount"
)

func TestLotusMount(t *testing.T) {
	ctx := context.Background()
	bgen := blocksutil.NewBlockGenerator()
	cid := bgen.Next().Cid()

	mockCtrl := gomock.NewController(t)
	// when test is done, assert expectations on all mock objects.
	defer mockCtrl.Finish()

	// create a mock lotus api that returns the reader we want
	mockLotusMountAPI := mock_dagstore2.NewMockLotusAccessor(mockCtrl)

	mockLotusMountAPI.EXPECT().IsUnsealed(gomock.Any(), cid).Return(true, nil).Times(1)

	mockLotusMountAPI.EXPECT().FetchUnsealedPiece(gomock.Any(), cid).Return(testReader(), nil).Times(1)
	mockLotusMountAPI.EXPECT().FetchUnsealedPiece(gomock.Any(), cid).Return(testReader(), nil).Times(1)
	mockLotusMountAPI.EXPECT().GetUnpaddedCARSize(ctx, cid).Return(uint64(100), nil).Times(1)

	mnt, err := NewPieceMount(cid, false, mockLotusMountAPI)
	require.NoError(t, err)
	info := mnt.Info()
	require.Equal(t, info.Kind, mount.KindRemote)

	// fetch and assert success
	rd, err := mnt.Fetch(context.Background())
	require.NoError(t, err)

	bz, err := ioutil.ReadAll(rd)
	require.NoError(t, err)
	require.NoError(t, rd.Close())
	require.Equal(t, []byte("testing"), bz)

	stat, err := mnt.Stat(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 100, stat.Size)

	// serialize url then deserialize from mount template -> should get back
	// the same mount
	url := mnt.Serialize()
	mnt2 := mountTemplate(mockLotusMountAPI, false)
	err = mnt2.Deserialize(url)
	require.NoError(t, err)

	// fetching on this mount should get us back the same data.
	rd, err = mnt2.Fetch(context.Background())
	require.NoError(t, err)
	bz, err = ioutil.ReadAll(rd)
	require.NoError(t, err)
	require.NoError(t, rd.Close())
	require.Equal(t, []byte("testing"), bz)
}

func TestLotusMountDeserialize(t *testing.T) {
	api := &marketAPI{}

	bgen := blocksutil.NewBlockGenerator()
	cid := bgen.Next().Cid()

	// success
	us := marketScheme + "://" + cid.String()
	u, err := url.Parse(us)
	require.NoError(t, err)

	mnt := mountTemplate(api, false)
	err = mnt.Deserialize(u)
	require.NoError(t, err)

	require.Equal(t, cid, mnt.PieceCid)
	require.Equal(t, api, mnt.API)

	// fails if cid is not valid
	us = marketScheme + "://" + "rand"
	u, err = url.Parse(us)
	require.NoError(t, err)
	err = mnt.Deserialize(u)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse PieceCid")
}

func TestLotusMountRegistration(t *testing.T) {
	ctx := context.Background()
	bgen := blocksutil.NewBlockGenerator()
	cid := bgen.Next().Cid()

	// success
	us := marketScheme + "://" + cid.String()
	u, err := url.Parse(us)
	require.NoError(t, err)

	mockCtrl := gomock.NewController(t)
	// when test is done, assert expectations on all mock objects.
	defer mockCtrl.Finish()

	mockLotusMountAPI := mock_dagstore2.NewMockLotusAccessor(mockCtrl)
	registry := mount.NewRegistry()
	err = registry.Register(marketScheme, mountTemplate(mockLotusMountAPI, false))
	require.NoError(t, err)

	mnt, err := registry.Instantiate(u)
	require.NoError(t, err)

	mockLotusMountAPI.EXPECT().IsUnsealed(ctx, cid).Return(true, nil)
	mockLotusMountAPI.EXPECT().GetUnpaddedCARSize(ctx, cid).Return(uint64(100), nil).Times(1)
	stat, err := mnt.Stat(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 100, stat.Size)
	require.True(t, stat.Ready)
}

func TestMarket2(t *testing.T) {
	ctx := context.Background()
	payloadSize := 644642936
	flen := abi.PaddedPieceSize(1073741824)
	assert.Nil(t, flen.Validate())
	testResourceId, _ := cid.Decode("baga6ea4seaqodfmewxaiqnf2sl26rrvub6wgyd6deiwitlgv2v363gg5fbemmli")

	testCId, _ := cid.Decode("bafy2bzacecqwr2ggwu62ao246wzilhba5dvbocjwxxwyb2zn3wl7rgk2wsx3k")
	memPieceStorage, err := piecestorage.NewFsPieceStorage(config.FsPieceStorage{true, "/Users/lijunlong/code/venus-market/dagstore/fixtures"})
	assert.Nil(t, err)
	r := models.NewInMemoryRepo()
	err = r.StorageDealRepo().SaveDeal(ctx, &market.MinerDeal{
		ClientDealProposal: builtinMarket.ClientDealProposal{
			Proposal: market0.DealProposal{
				Provider:  address.TestAddress,
				Client:    address.TestAddress,
				PieceCID:  testResourceId,
				PieceSize: flen,
			},
			ClientSignature: acrypto.Signature{
				Type: acrypto.SigTypeBLS,
				Data: make([]byte, 10),
			},
		},
		ProposalCid: testCId,
		PayloadSize: uint64(payloadSize),
	})
	assert.Nil(t, err)

	marketAPI := NewMarketAPI(r, memPieceStorage, false)
	pr, err := marketAPI.FetchUnsealedPiece(ctx, testResourceId)
	assert.Nil(t, err)

	generateIndex, err := car.ReadOrGenerateIndex(pr, car.ZeroLengthSectionAsEOF(true))
	assert.Nil(t, err)
	actualBuf := bytes.NewBuffer([]byte{})
	carindex.WriteTo(generateIndex, actualBuf)

	expectIndex, err := os.Open("/Users/lijunlong/code/venus-market/dagstore/fixtures/index/baga6ea4seaqodfmewxaiqnf2sl26rrvub6wgyd6deiwitlgv2v363gg5fbemmli.full.idx")
	assert.Nil(t, err)
	expectIndexBytes, err := ioutil.ReadAll(expectIndex)
	assert.Nil(t, err)

	actualBytes := actualBuf.Bytes()
	assert.Equal(t, actualBytes, expectIndexBytes)
}

func testReader() mount.Reader {
	r := bytes.NewReader([]byte("testing"))
	return utils.WrapCloser{r, r}
}
