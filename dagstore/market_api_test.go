package dagstore

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin/v8/market"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	markettypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestMarket(t *testing.T) {
	ctx := context.Background()
	payloadSize := 10
	flen := abi.PaddedPieceSize(128)
	assert.Nil(t, flen.Validate())
	testResourceId, _ := cid.Decode("baga6ea4seaqd6cvb2padh74lthhiay4jtlwqhj2qetbj5cipna6jlkmcrdljulq")

	testCId, _ := cid.Decode("bafy2bzacecqwr2ggwu62ao246wzilhba5dvbocjwxxwyb2zn3wl7rgk2wsx3k")
	memPieceStorage := piecestorage.NewMemPieceStore("", nil)
	pmgr, err := piecestorage.NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	pmgr.AddMemPieceStorage(memPieceStorage)

	r := models.NewInMemoryRepo()
	err = r.StorageDealRepo().SaveDeal(ctx, &markettypes.MinerDeal{
		ClientDealProposal: market.ClientDealProposal{
			Proposal: market.DealProposal{
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
	payloadWriter := bytes.NewBufferString(strings.Repeat("1", payloadSize))
	_, err = memPieceStorage.SaveTo(ctx, testResourceId.String(), payloadWriter)
	assert.Nil(t, err)

	marketAPI := NewMarketAPI(ctx, r, pmgr, false)

	size, err := marketAPI.GetUnpaddedCARSize(ctx, testResourceId)
	assert.Nil(t, err)
	assert.Equal(t, uint64(flen), size)

	_, err = marketAPI.FetchFromPieceStorage(ctx, testResourceId)
	assert.Nil(t, err)
}
