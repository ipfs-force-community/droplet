package badger

import (
	"context"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	market8 "github.com/filecoin-project/go-state-types/builtin/v8/market"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"

	"github.com/stretchr/testify/assert"

	m_types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func init() {
	testutil.MustRegisterDefaultValueProvier(func(t *testing.T) market8.DealLabel {
		l, err := market8.NewLabelFromBytes([]byte{})
		assert.NoError(t, err)
		return l
	})
}

func TestStorageDeal(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.StorageDealRepo()

	dealCases := make([]m_types.MinerDeal, 10)
	testutil.Provide(t, &dealCases)

	dealCases[0].PieceStatus = m_types.Assigned
	t.Run("SaveDeal", func(t *testing.T) {
		for _, deal := range dealCases {
			err := r.SaveDeal(ctx, &deal)
			assert.NoError(t, err)
		}
	})

	t.Run("GetDeal", func(t *testing.T) {
		res, err := r.GetDeal(ctx, dealCases[0].ProposalCid)
		assert.NoError(t, err)
		dealCases[0].UpdatedAt = res.UpdatedAt
		dealCases[0].CreationTime = res.CreationTime
		assert.Equal(t, dealCases[0], *res)
	})

	// refresh UpdatedAt and CreationTime
	for i := range dealCases {
		res, err := r.GetDeal(ctx, dealCases[i].ProposalCid)
		assert.NoError(t, err)
		dealCases[i].UpdatedAt = res.UpdatedAt
		dealCases[i].CreationTime = res.CreationTime
	}

	t.Run("ListDeal", func(t *testing.T) {
		res, err := r.ListDeal(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(dealCases), len(res))
		for _, deal := range res {
			assert.Contains(t, dealCases, *deal)
		}
	})

	t.Run("GetDealByDealID", func(t *testing.T) {
		res, err := r.GetDealByDealID(ctx, dealCases[0].Proposal.Provider, dealCases[0].DealID)
		assert.NoError(t, err)
		assert.Equal(t, dealCases[0], *res)
	})

	t.Run("GetDeals", func(t *testing.T) {
		res, err := r.GetDeals(ctx, dealCases[0].Proposal.Provider, 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("GetDealsByPieceCidAndStatus", func(t *testing.T) {
		res, err := r.GetDealsByPieceCidAndStatus(ctx, dealCases[0].Proposal.PieceCID, dealCases[0].State)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("GetDealsByPieceStatusAndDealStatus", func(t *testing.T) {
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
	})

	t.Run("GetDealsByDataCidAndDealStatus", func(t *testing.T) {
		t.Run("With Provider", func(t *testing.T) {
			res, err := r.GetDealsByDataCidAndDealStatus(ctx, dealCases[0].Proposal.Provider, dealCases[0].Ref.Root, []m_types.PieceStatus{dealCases[0].PieceStatus})
			assert.NoError(t, err)
			assert.Equal(t, 1, len(res))
			assert.Equal(t, dealCases[0], *res[0])
		})

		t.Run("Without Provider", func(t *testing.T) {
			res, err := r.GetDealsByDataCidAndDealStatus(ctx, address.Undef, dealCases[0].Ref.Root, []m_types.PieceStatus{dealCases[0].PieceStatus})
			assert.NoError(t, err)
			assert.Equal(t, 1, len(res))
			assert.Equal(t, dealCases[0], *res[0])
		})
	})

	t.Run("GetDealByAddrAndStatus", func(t *testing.T) {
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
	})

	t.Run("ListDealByAddr", func(t *testing.T) {
		res, err := r.ListDealByAddr(ctx, dealCases[0].Proposal.Provider)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, dealCases[0], *res[0])
	})

	t.Run("GetPieceInfo", func(t *testing.T) {
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
	})

	t.Run("ListPieceInfoKeys", func(t *testing.T) {
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
	})

	t.Run("GetPieceSize", func(t *testing.T) {
		PLSize, PSize, err := r.GetPieceSize(ctx, dealCases[0].Proposal.PieceCID)
		assert.NoError(t, err)
		assert.Equal(t, dealCases[0].Proposal.PieceSize, PSize)
		assert.Equal(t, dealCases[0].PayloadSize, PLSize)
	})

	t.Run("UpdateDealStatus", func(t *testing.T) {
		err := r.UpdateDealStatus(ctx, dealCases[0].ProposalCid, storagemarket.StorageDealActive, m_types.Proving)
		assert.NoError(t, err)
		res, err := r.GetDeal(ctx, dealCases[0].ProposalCid)
		assert.NoError(t, err)
		assert.Equal(t, storagemarket.StorageDealActive, res.State)
		assert.Equal(t, m_types.Proving, res.PieceStatus)
	})

	t.Run("GroupStorageDealNumberByStatus", func(t *testing.T) {
		t.Run("correct", func(t *testing.T) {
			repo := setup(t)
			r := repo.StorageDealRepo()

			deals := make([]m_types.MinerDeal, 100)
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

			deals := make([]m_types.MinerDeal, 10)
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

	})

}
