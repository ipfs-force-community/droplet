package models

import (
	"github.com/filecoin-project/venus-market/types"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/venus-market/models/badger"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/stretchr/testify/assert"
	typegen "github.com/whyrusleeping/cbor-gen"
)

func TestStorageDeal(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		testStorageDeal(t, MysqlDB(t).StorageDealRepo())
	})

	t.Run("badger", func(t *testing.T) {
		path := "./badger_storage_deal_db"
		db := BadgerDB(t, path)
		defer func() {
			assert.Nil(t, db.Close())
			assert.Nil(t, os.RemoveAll(path))

		}()
		testStorageDeal(t, repo.StorageDealRepo(badger.NewStorageDealRepo(db)))
	})
}

func testStorageDeal(t *testing.T, dealRepo repo.StorageDealRepo) {
	c := randCid(t)
	pid, err := peer.Decode("12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd")
	if err != nil {
		assert.Nil(t, err)
	}

	deal := &types.MinerDeal{
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
		TransferChannelId:     nil,
		SectorNumber:          10,
		InboundCAR:            "InboundCAR",
		Offset: 1022222,
		Length: 555,
	}
	assert.Nil(t, dealRepo.SaveDeal(deal))

	deal2 := &types.MinerDeal{}
	*deal2 = *deal
	deal2.ProposalCid = randCid(t)
	deal2.TransferChannelId = &datatransfer.ChannelID{
		Initiator: pid,
		Responder: pid,
		ID:        10,
	}
	deal2.Proposal.Provider = randAddress(t)
	assert.Nil(t, dealRepo.SaveDeal(deal2))

	res, err := dealRepo.GetDeal(deal.ProposalCid)
	assert.Nil(t, err)
	compareDeal(t, res, deal)
	res2, err := dealRepo.GetDeal(deal2.ProposalCid)
	assert.Nil(t, err)
	compareDeal(t, res2, deal2)

	list, err := dealRepo.ListDeal(deal.Proposal.Provider)
	assert.Nil(t, err)
	assert.Equal(t, len(list), 1)
	compareDeal(t, list[0], deal)
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
	assert.Equal(t, actual.TransferChannelId, excepted.TransferChannelId)
	assert.Equal(t, excepted.SectorNumber, actual.SectorNumber)
	assert.Equal(t, excepted.InboundCAR, actual.InboundCAR)
	assert.Equal(t, excepted.Offset, actual.Offset)
	assert.Equal(t, excepted.Length, actual.Length)
}
