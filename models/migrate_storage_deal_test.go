package models

import (
	"context"
	"strings"
	"testing"
	"time"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"github.com/filecoin-project/venus-market/models/badger"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	typegen "github.com/whyrusleeping/cbor-gen"
)

func TestMigrateStorageDeal(t *testing.T) {
	ctx := context.Background()
	ds := datastore.NewMapDatastore()
	storageDealDS := badger.NewStorageDealsDS(ds)
	deal := getMinerDealV0(t)
	b, err := cborrpc.Dump(deal)
	assert.Nil(t, err)
	assert.Nil(t, storageDealDS.Put(ctx, statestore.ToKey(deal.ProposalCid), b))

	dataRef := &types.DataRef{
		TransferType: deal.Ref.TransferType,
		Root:         deal.Ref.Root,
		PieceCid:     deal.Ref.PieceCid,
		PieceSize:    deal.Ref.PieceSize,
		RawBlockSize: deal.Ref.RawBlockSize,
	}
	r, err := badger.NewBadgerRepo(badger.BadgerDSParams{StorageDealsDS: storageDealDS})
	assert.Nil(t, err)
	res2, err := r.StorageDealRepo().GetDeal(ctx, deal.ProposalCid)
	assert.Nil(t, err)
	assert.Equal(t, dataRef, res2.Ref)

	_, err = storageDealDS.Get(ctx, statestore.ToKey(deal.ProposalCid))
	assert.True(t, strings.Contains(err.Error(), "not found"))

	version, err := storageDealDS.Get(ctx, datastore.NewKey("versions/current"))
	assert.Nil(t, err)
	assert.Equal(t, "1", string(version))
}

func getMinerDealV0(t *testing.T) *types.MinerDealV0 {
	c := randCid(t)
	pid, err := peer.Decode("12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd")
	if err != nil {
		assert.Nil(t, err)
	}
	return &types.MinerDealV0{
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
