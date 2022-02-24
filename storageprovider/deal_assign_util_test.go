package storageprovider

import (
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	mtypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/require"
)

const (
	isDeal  = abi.DealID(1)
	nonDeal = abi.DealID(0)
)

func generateTestingDeals(sizes []abi.PaddedPieceSize) []*mtypes.DealInfoIncludePath {
	deals := make([]*mtypes.DealInfoIncludePath, len(sizes))
	for si, size := range sizes {
		deals[si] = &mtypes.DealInfoIncludePath{
			DealID: isDeal,
			DealProposal: market.DealProposal{
				PieceSize: size,
			},
		}
	}

	return deals
}

func TestDealAssignPickAndAlign(t *testing.T) {
	const SectorSize2K = abi.SectorSize(2 << 10)

	cases := []struct {
		sectorSize        abi.SectorSize
		sizes             []abi.PaddedPieceSize
		expectedDealIDs   []abi.DealID
		expectedPieceSize []abi.PaddedPieceSize
		expectedErr       error
	}{
		// ## Common Cases
		// 128
		{
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128},
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		// 128 + 256
		{
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 256},
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, isDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		// ## Err Cases
		// invalid sector size
		{
			sectorSize:  SectorSize2K + 1,
			sizes:       []abi.PaddedPieceSize{128},
			expectedErr: errInvalidSpaceSize,
		},

		// invalid piece size
		{
			sectorSize:  SectorSize2K,
			sizes:       []abi.PaddedPieceSize{128, 257},
			expectedErr: errInvalidDealPieceSize,
		},

		// un-ordered
		{
			sectorSize:  SectorSize2K,
			sizes:       []abi.PaddedPieceSize{256, 128},
			expectedErr: errDealsUnOrdered,
		},

		// Edge Cases
		// big deal

		{
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{512 << 30},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		// Ohter Valid Cases
		{
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 512, 512},
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal, isDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 512, 512},
			expectedErr:       nil,
		},

		{
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},
	}

	for ci := range cases {
		c := cases[ci]
		expectedPieceCount := len(c.expectedPieceSize)
		require.Lenf(t, c.expectedDealIDs, expectedPieceCount, "#%d expected deal ids & piece sizes should be equal", ci)

		caseDeals := generateTestingDeals(c.sizes)
		gotDeals, gotErr := pickAndAlign(caseDeals, c.sectorSize)

		if c.expectedErr != nil {
			require.ErrorIsf(t, gotErr, c.expectedErr, "#%d expected a specified error", ci)
			continue
		}

		require.NoErrorf(t, gotErr, "#%d case should be valid", ci)
		require.Lenf(t, gotDeals, expectedPieceCount, "#%d result deals count", ci)

		for i := range gotDeals {
			require.Equalf(t, c.expectedDealIDs[i], gotDeals[i].DealID, "#%d id of deal %d not match", ci, i)
			require.Equalf(t, c.expectedPieceSize[i], gotDeals[i].PieceSize, "#%d piece size of deal %d not match", ci, i)
		}
	}
}
