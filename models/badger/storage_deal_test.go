package badger

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/types"
	markettypes "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func init() {
	testutil.MustRegisterDefaultValueProvier(func(t *testing.T) types.DealLabel {
		l, err := types.NewLabelFromBytes([]byte{})
		assert.NoError(t, err)
		return l
	})
}

func prepareStorageDealTest(t *testing.T) (context.Context, repo.StorageDealRepo, []markettypes.MinerDeal) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.StorageDealRepo()

	dealCases := make([]markettypes.MinerDeal, 10)
	testutil.Provide(t, &dealCases)
	dealCases[0].PieceStatus = markettypes.Assigned
	return ctx, r, dealCases
}

func TestCreateStorageDeals(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	deals := make([]*markettypes.MinerDeal, 0, len(dealCases))
	for i := range dealCases {
		deals = append(deals, &dealCases[i])
	}
	assert.NoError(t, r.CreateDeals(ctx, deals))

	ret, err := r.ListDeal(ctx, &markettypes.StorageDealQueryParams{Page: markettypes.Page{Limit: 11}})
	assert.NoError(t, err)
	assert.Len(t, ret, 10)
}

func TestSaveStorageDeal(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}
}

func TestGetStorageDeal(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	res, err := r.GetDeal(ctx, dealCases[0].ProposalCid)
	assert.NoError(t, err)
	dealCases[0].UpdatedAt = res.UpdatedAt
	dealCases[0].CreationTime = res.CreationTime
	assert.Equal(t, dealCases[0], *res)
}

func TestListStorageDeal(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	res, err := r.ListDeal(ctx, &markettypes.StorageDealQueryParams{Page: markettypes.Page{Limit: math.MaxInt32}})
	assert.NoError(t, err)
	assert.Equal(t, len(dealCases), len(res))
	for _, deal := range res {
		assert.Contains(t, dealCases, *deal)
	}
}

func TestGetStorageDealByDealID(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	res, err := r.GetDealByDealID(ctx, dealCases[0].Proposal.Provider, dealCases[0].DealID)
	assert.NoError(t, err)
	assert.Equal(t, dealCases[0], *res)
}

func TestGetStorageDeals(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}
	res, err := r.GetDeals(ctx, dealCases[0].Proposal.Provider, 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, dealCases[0], *res[0])
}

func TestGetStorageDealsByPieceCidAndStatus(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	res, err := r.GetDealsByPieceCidAndStatus(ctx, dealCases[0].Proposal.PieceCID, dealCases[0].State)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, dealCases[0], *res[0])
}

func TestGetStorageDealsByPieceStatusAndDealStatus(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	t.Run("With DealStatus", func(t *testing.T) {
		res, err := r.GetDealsByPieceStatusAndDealStatus(ctx, dealCases[0].Proposal.Provider, dealCases[0].PieceStatus, dealCases[0].State)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("Without DealStatus", func(t *testing.T) {
		res, err := r.GetDealsByPieceStatusAndDealStatus(ctx, dealCases[0].Proposal.Provider, dealCases[0].PieceStatus)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("Without Provider", func(t *testing.T) {
		res, err := r.GetDealsByPieceStatusAndDealStatus(ctx, address.Undef, dealCases[0].PieceStatus, dealCases[0].State)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("Will Return None", func(t *testing.T) {
		res, err := r.GetDealsByPieceStatusAndDealStatus(ctx, address.Undef, dealCases[0].PieceStatus, 0)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(res))
	})
}

func TestGetStorageDealsByDataCidAndDealStatus(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	t.Run("With Provider", func(t *testing.T) {
		res, err := r.GetDealsByDataCidAndDealStatus(ctx, dealCases[0].Proposal.Provider, dealCases[0].Ref.Root, []markettypes.PieceStatus{dealCases[0].PieceStatus})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("Without Provider", func(t *testing.T) {
		res, err := r.GetDealsByDataCidAndDealStatus(ctx, address.Undef, dealCases[0].Ref.Root, []markettypes.PieceStatus{dealCases[0].PieceStatus})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})
}

func TestGetStorageDealByAddrAndStatus(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	t.Run("With Provider", func(t *testing.T) {
		res, err := r.GetDealByAddrAndStatus(ctx, dealCases[0].Proposal.Provider, dealCases[0].State)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("Without Provider", func(t *testing.T) {
		res, err := r.GetDealByAddrAndStatus(ctx, address.Undef, dealCases[0].State)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})
}

func TestListStorageDealByAddr(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	res, err := r.ListDealByAddr(ctx, dealCases[0].Proposal.Provider)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, dealCases[0], *res[0])
}

func TestListDeal(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	peers := []peer.ID{peer.ID("1"), peer.ID("2")}
	byPiece := make(map[string]int)
	miner := []address.Address{dealCases[0].Proposal.Provider, testutil.AddressProvider()(t)}
	states := []storagemarket.StorageDealStatus{
		storagemarket.StorageDealAcceptWait,
		storagemarket.StorageDealAwaitingPreCommit,
		storagemarket.StorageDealFailing,
		storagemarket.StorageDealExpired,
	}

	assert.NotEqual(t, peers[0].String(), peers[1].String())

	for i, deal := range dealCases {
		deal.Proposal.Provider = miner[i%2]
		deal.Client = peers[i%2]
		deal.State = states[i%4]
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
		byPiece[deal.Proposal.PieceCID.String()]++
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	defPage := markettypes.Page{Limit: len(dealCases)}

	// params is empty
	deals, err := r.ListDeal(ctx, &markettypes.StorageDealQueryParams{})
	assert.NoError(t, err)
	assert.Len(t, deals, 0)

	// test page
	deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
		Page: markettypes.Page{
			Limit: 3,
		},
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)
	deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
		Page: markettypes.Page{
			Offset: len(dealCases) - 3,
			Limit:  4,
		},
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)

	for i := 0; i < 2; i++ {
		deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
			Miner: miner[i],
			Page:  defPage,
		})
		assert.NoError(t, err)
		assert.Len(t, deals, 5)

		deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
			Client: peers[i].Pretty(),
			Page:   defPage,
		})
		assert.NoError(t, err)
		assert.Len(t, deals, 5)
	}

	storageDealAcceptWait := storagemarket.StorageDealAcceptWait
	deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
		State: &storageDealAcceptWait,
		Page:  defPage,
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 3)

	deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
		DiscardFailedDeal: true,
		Page:              defPage,
	})
	assert.NoError(t, err)
	assert.Equal(t, 6, len(deals))

	storageDealFailing := storagemarket.StorageDealFailing
	deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
		State:             &storageDealFailing,
		DiscardFailedDeal: true,
		Page:              defPage,
	})
	assert.NoError(t, err)
	assert.Len(t, deals, 2)

	// test piece
	for piece, count := range byPiece {
		deals, err = r.ListDeal(ctx, &markettypes.StorageDealQueryParams{
			Page: markettypes.Page{
				Limit: 100,
			},
			PieceCID: piece,
		})
		assert.NoError(t, err)
		assert.Len(t, deals, count)
	}

}

func TestGetStoragePieceInfo(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	res, err := r.GetPieceInfo(ctx, dealCases[0].Proposal.PieceCID)
	assert.NoError(t, err)
	expect := piecestore.PieceInfo{
		PieceCID: dealCases[0].Proposal.PieceCID,
		Deals:    nil,
	}
	expect.Deals = append(expect.Deals, piecestore.DealInfo{
		DealID:   dealCases[0].DealID,
		SectorID: dealCases[0].SectorNumber,
		Offset:   dealCases[0].Offset,
		Length:   dealCases[0].Proposal.PieceSize,
	})
	assert.Equal(t, expect, *res)
}

func TestListPieceInfoKeys(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	res, err := r.ListPieceInfoKeys(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(dealCases), len(res))
	exp := make([]cid.Cid, 0, len(dealCases))
	for _, deal := range dealCases {
		exp = append(exp, deal.Proposal.PieceCID)
	}
	for _, id := range res {
		assert.Contains(t, exp, id)
	}
}

func TestGetPieceSize(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	PLSize, PSize, err := r.GetPieceSize(ctx, dealCases[0].Proposal.PieceCID)
	assert.NoError(t, err)
	assert.Equal(t, dealCases[0].Proposal.PieceSize, PSize)
	assert.Equal(t, dealCases[0].PayloadSize, PLSize)
}

func TestUpdateStorageDealStatus(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	err := r.UpdateDealStatus(ctx, dealCases[0].ProposalCid, storagemarket.StorageDealActive, markettypes.Proving)
	assert.NoError(t, err)
	res, err := r.GetDeal(ctx, dealCases[0].ProposalCid)
	assert.NoError(t, err)
	assert.Equal(t, storagemarket.StorageDealActive, res.State)
	assert.Equal(t, markettypes.Proving, res.PieceStatus)
}

func TestGroupStorageDealNumberByStatus(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)
	}

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	t.Run("correct", func(t *testing.T) {
		repo := setup(t)
		r := repo.StorageDealRepo()

		deals := make([]markettypes.MinerDeal, 100)
		testutil.Provide(t, &deals)

		var addrs []address.Address
		addrGetter := address.NewForTestGetter()
		for i := 0; i < 3; i++ {
			addrs = append(addrs, addrGetter())
		}

		for index := range deals {
			deals[index].ClientDealProposal.Proposal.Provider = addrs[rand.Intn(len(addrs))]
			deals[index].State = storagemarket.StorageDealStatus(rand.Intn(int(storagemarket.StorageDealReserveProviderFunds)))
		}

		for _, deal := range deals {
			err := r.SaveDeal(ctx, &deal)
			assert.Nil(t, err)
		}

		result := map[storagemarket.StorageDealStatus]int64{}
		for _, deal := range deals {
			if deal.Proposal.Provider != addrs[0] {
				continue
			}
			result[deal.State]++
		}
		result2, err := r.GroupStorageDealNumberByStatus(ctx, addrs[0])
		assert.Nil(t, err)
		assert.Equal(t, result, result2)
	})

	t.Run("undefined address", func(t *testing.T) {
		repo := setup(t)
		r := repo.StorageDealRepo()

		deals := make([]markettypes.MinerDeal, 10)
		testutil.Provide(t, &deals)

		result := map[storagemarket.StorageDealStatus]int64{}
		for _, deal := range deals {
			result[deal.State]++
		}

		for _, deal := range deals {
			err := r.SaveDeal(ctx, &deal)
			assert.Nil(t, err)
		}

		result2, err := r.GroupStorageDealNumberByStatus(ctx, address.Undef)
		assert.Nil(t, err)
		assert.Equal(t, result, result2)
	})
}

func TestGetStorageDealByUUID(t *testing.T) {
	ctx, r, dealCases := prepareStorageDealTest(t)

	for _, deal := range dealCases {
		err := r.SaveDeal(ctx, &deal)
		assert.NoError(t, err)

		res, err := r.GetDealByUUID(ctx, deal.ID)
		assert.NoError(t, err)
		deal.UpdatedAt = res.UpdatedAt
		deal.CreationTime = res.CreationTime
		assert.Equal(t, &deal, res)
	}

	res, err := r.GetDealByUUID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Nil(t, res)
}
