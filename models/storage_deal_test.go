package models

import (
	"bytes"
	"context"
	"testing"
	"time"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/require"

	"github.com/libp2p/go-libp2p-core/peer"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/stretchr/testify/assert"
	typegen "github.com/whyrusleeping/cbor-gen"
)

func TestStorageDeal(t *testing.T) {
	t.Run("MinerDealMarshal", testCborMarshal)

	t.Run("mysql", func(t *testing.T) {
		repo := MysqlDB(t)
		dealRepo := repo.StorageDealRepo()
		defer func() {
			_ = repo.Close()
		}()
		testStorageDeal(t, dealRepo)
	})

	t.Run("badger", func(t *testing.T) {
		db := BadgerDB(t)
		testStorageDeal(t, badger.NewStorageDealRepo(db))
	})
}

func getTestMinerDeal(t *testing.T) *types.MinerDeal {
	c := randCid(t)
	pid, err := peer.Decode("12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd")
	if err != nil {
		assert.Nil(t, err)
	}
	return &types.MinerDeal{
		ClientDealProposal: market.ClientDealProposal{
			Proposal: market.DealProposal{
				PieceCID:             c,
				PieceSize:            1024,
				VerifiedDeal:         false,
				Client:               randAddress(t),
				Provider:             randAddress(t),
				Label:                "label",
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
		State:         0,
		PiecePath:     "path",
		MetadataPath:  "path",
		SlashEpoch:    10,
		FastRetrieval: false,
		Message:       "message",
		FundsReserved: abi.NewTokenAmount(100),
		Ref: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         c,
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
	}
}

func testCborMarshal(t *testing.T) {
	src := getTestMinerDeal(t)

	buf := bytes.NewBuffer(nil)
	var dist types.MinerDeal
	require.NoError(t, src.MarshalCBOR(buf))
	require.NoError(t, dist.UnmarshalCBOR(buf))
	compareDeal(t, src, &dist)
}

func testStorageDeal(t *testing.T, dealRepo repo.StorageDealRepo) {
	ctx := context.TODO()
	pid, err := peer.Decode("12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd")
	if err != nil {
		assert.Nil(t, err)
	}

	deal := getTestMinerDeal(t)
	// test save and get
	assert.Nil(t, dealRepo.SaveDeal(ctx, deal))
	deal2, err := dealRepo.GetDeal(ctx, deal.ProposalCid)
	require.NoError(t, err)
	compareDeal(t, deal, deal2)
	assert.Nil(t, dealRepo.SaveDeal(ctx, deal2))

	// test update
	deal.Offset = 90000
	assert.Nil(t, dealRepo.SaveDeal(ctx, deal))

	deal2, err = dealRepo.GetDeal(ctx, deal.ProposalCid)
	require.NoError(t, err)
	compareDeal(t, deal, deal2)

	deal2.ProposalCid = randCid(t)
	deal2.TransferChannelID = &datatransfer.ChannelID{
		Initiator: pid,
		Responder: pid,
		ID:        10,
	}
	deal2.Proposal.Provider = randAddress(t)
	assert.Nil(t, dealRepo.SaveDeal(ctx, deal2))

	res, err := dealRepo.GetDeal(ctx, deal.ProposalCid)
	assert.Nil(t, err)
	compareDeal(t, res, deal)

	res2, err := dealRepo.GetDeal(ctx, deal2.ProposalCid)
	assert.Nil(t, err)
	compareDeal(t, res2, deal2)

	// test list
	list, err := dealRepo.ListDeal(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(list))

	_, err = dealRepo.GetDeal(ctx, randCid(t))
	require.Error(t, err, "recode shouldn't be found")

	pieceCids, err := dealRepo.ListPieceInfoKeys(ctx)
	assert.Nil(t, err)
	assert.Len(t, pieceCids, 1)
}

func compareDeal(t *testing.T, actual, excepted *types.MinerDeal) {
	assert.Equal(t, excepted.ClientDealProposal, actual.ClientDealProposal)
	assert.Equal(t, excepted.ProposalCid, actual.ProposalCid)
	assert.Equal(t, excepted.PublishCid, actual.PublishCid)
	assert.Equal(t, excepted.Miner, actual.Miner)
	assert.Equal(t, excepted.Client, actual.Client)
	assert.Equal(t, excepted.State, actual.State)
	assert.Equal(t, excepted.PiecePath, actual.PiecePath)
	assert.Equal(t, excepted.MetadataPath, actual.MetadataPath)
	assert.Equal(t, excepted.SlashEpoch, actual.SlashEpoch)
	assert.Equal(t, excepted.FastRetrieval, actual.FastRetrieval)
	assert.Equal(t, excepted.Message, actual.Message)
	assert.Equal(t, excepted.FundsReserved, actual.FundsReserved)
	assert.Equal(t, excepted.Ref, actual.Ref)
	assert.Equal(t, excepted.AvailableForRetrieval, actual.AvailableForRetrieval)
	assert.Equal(t, excepted.DealID, actual.DealID)
	assert.Equal(t, excepted.CreationTime.Time().UTC(), actual.CreationTime.Time().UTC())
	assert.Equal(t, actual.TransferChannelID, excepted.TransferChannelID)
	assert.Equal(t, excepted.SectorNumber, actual.SectorNumber)
	assert.Equal(t, excepted.InboundCAR, actual.InboundCAR)
	assert.Equal(t, excepted.Offset, actual.Offset)
}
