package retrievalprovider

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	typegen "github.com/whyrusleeping/cbor-gen"

	builtinMarket "github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus-market/v2/dagstore"

	market2 "github.com/filecoin-project/go-state-types/builtin/v8/market"
	"github.com/filecoin-project/venus-market/v2/models"
)

func TestPieceInfo_GetPieceInfoByPieceCid(t *testing.T) {
	ctx := context.Background()
	storageDealRepo := models.NewInMemoryRepo().StorageDealRepo()
	dagStore := dagstore.NewMockDagStoreWrapper()
	pieceStore := PieceInfo{
		dagstore: dagStore,
		dealRepo: storageDealRepo,
	}
	dataCid := randCid(t)
	mockPieceCid := randCid(t)
	err := storageDealRepo.SaveDeal(ctx, getTestMinerDeal(t, dataCid, mockPieceCid))
	assert.Nil(t, err)

	deals, err := pieceStore.GetPieceInfoFromCid(ctx, dataCid, &mockPieceCid)
	assert.Nil(t, err)
	assert.Len(t, deals, 1)
}

func TestPieceInfo_GetPieceInfoWithUnkownPieceCid(t *testing.T) {
	ctx := context.Background()
	storageDealRepo := models.NewInMemoryRepo().StorageDealRepo()
	dagStore := dagstore.NewMockDagStoreWrapper()
	pieceStore := PieceInfo{
		dagstore: dagStore,
		dealRepo: storageDealRepo,
	}
	dataCid := randCid(t)
	mockPieceCid := randCid(t)

	_, err := pieceStore.GetPieceInfoFromCid(ctx, dataCid, &mockPieceCid)
	assert.Equal(t, repo.ErrNotFound, err)
}

func TestPieceInfo_GetPieceInfoWithUnko(t *testing.T) {
	ctx := context.Background()
	storageDealRepo := models.NewInMemoryRepo().StorageDealRepo()
	dagStore := dagstore.NewMockDagStoreWrapper()
	pieceStore := PieceInfo{
		dagstore: dagStore,
		dealRepo: storageDealRepo,
	}
	dataCid := randCid(t)
	blockCId := randCid(t)
	mockPieceCid := randCid(t)

	err := storageDealRepo.SaveDeal(ctx, getTestMinerDeal(t, dataCid, mockPieceCid))
	assert.Nil(t, err)

	dagStore.AddBlockToPieceIndex(blockCId, mockPieceCid)
	deals, err := pieceStore.GetPieceInfoFromCid(ctx, blockCId, nil)
	assert.Nil(t, err)
	assert.Len(t, deals, 1)
}

func TestPieceInfo_DistinctDeals(t *testing.T) {
	ctx := context.Background()
	storageDealRepo := models.NewInMemoryRepo().StorageDealRepo()
	dagStore := dagstore.NewMockDagStoreWrapper()
	pieceStore := PieceInfo{
		dagstore: dagStore,
		dealRepo: storageDealRepo,
	}
	dataCid := randCid(t)
	mockPieceCid := randCid(t)

	err := storageDealRepo.SaveDeal(ctx, getTestMinerDeal(t, dataCid, mockPieceCid))
	assert.Nil(t, err)

	dagStore.AddBlockToPieceIndex(dataCid, mockPieceCid)
	deals, err := pieceStore.GetPieceInfoFromCid(ctx, dataCid, nil)
	assert.Nil(t, err)
	assert.Len(t, deals, 1)
}

func getTestMinerDeal(t *testing.T, datacid, pieceCid cid.Cid) *market.MinerDeal {
	c := randCid(t)
	pid, err := peer.Decode("12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd")
	assert.Nil(t, err)

	return &market.MinerDeal{
		ClientDealProposal: market2.ClientDealProposal{
			Proposal: builtinMarket.DealProposal{
				PieceCID:             pieceCid,
				PieceSize:            1024,
				VerifiedDeal:         false,
				Client:               randAddress(t),
				Provider:             randAddress(t),
				Label:                market2.DealLabel{},
				StartEpoch:           10,
				EndEpoch:             10,
				StoragePricePerEpoch: abi.NewTokenAmount(10),
				ProviderCollateral:   abi.NewTokenAmount(10),
				ClientCollateral:     abi.NewTokenAmount(101),
			},
			ClientSignature: crypto.Signature{
				Type: crypto.SigTypeBLS,
				Data: []byte("bls"),
			},
		},
		ProposalCid:   randCid(t),
		AddFundsCid:   &c,
		PublishCid:    &c,
		Miner:         pid,
		Client:        pid,
		State:         storagemarket.StorageDealAwaitingPreCommit,
		PiecePath:     "path",
		MetadataPath:  "path",
		SlashEpoch:    10,
		FastRetrieval: false,
		Message:       "message",
		FundsReserved: abi.NewTokenAmount(100),
		Ref: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         datacid,
			PieceCid:     &c,
			PieceSize:    1024,
			RawBlockSize: 1024,
		},
		AvailableForRetrieval: false,
		DealID:                10,
		CreationTime:          typegen.CborTime(time.Unix(0, time.Now().UnixNano()).UTC()),
		TransferChannelID:     nil,
		SectorNumber:          10,
		InboundCAR:            "InboundCAR",
		Offset:                1022222,
		PieceStatus:           "Proving",
	}
}

func randCid(t *testing.T) cid.Cid {
	totalLen := 62
	b := bytes.Buffer{}
	data := []byte("bafy2bzacedfra7y3yb5feuxm3iizqubo3jufhrwfw6yy74")
	_, err := b.Write(data)
	assert.Nil(t, err)
	for i := 0; i < totalLen-len(data); i++ {
		idx := rand.Intn(len(data))
		assert.Nil(t, b.WriteByte(data[idx]))
	}

	id, err := cid.Decode(b.String())
	assert.Nil(t, err)
	return id
}

func randAddress(t *testing.T) address.Address {
	addr, err := address.NewActorAddress([]byte(uuid.New().String()))
	if err != nil {
		t.Fatal(err)
	}
	return addr
}
