package storageprovider

import (
	market7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
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
			DealProposal: market7.DealProposal{
				PieceSize: size,
			},
		}
	}

	return deals
}

func TestDealAssignPickAndAlign(t *testing.T) {
	const SectorSize2K = abi.SectorSize(2 << 10)

	cases := []struct {
		name              string
		sectorSize        abi.SectorSize
		sizes             []abi.PaddedPieceSize
		spec              *mtypes.GetDealSpec
		expectedDealIDs   []abi.DealID
		expectedPieceSize []abi.PaddedPieceSize
		expectedErr       error
	}{
		// ## Common Cases
		{
			name:              "128",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "128 + 256",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 256},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, isDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "2048",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{2048},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal},
			expectedPieceSize: []abi.PaddedPieceSize{2048},
			expectedErr:       nil,
		},

		// ## Err Cases
		{
			name:        "invalid sector size",
			sectorSize:  SectorSize2K + 1,
			sizes:       []abi.PaddedPieceSize{128},
			spec:        nil,
			expectedErr: errInvalidSpaceSize,
		},

		{
			name:        "invalid piece size",
			sectorSize:  SectorSize2K,
			sizes:       []abi.PaddedPieceSize{128, 257},
			spec:        nil,
			expectedErr: errInvalidDealPieceSize,
		},

		// un-ordered
		{
			name:        "pieces un-ordered",
			sectorSize:  SectorSize2K,
			sizes:       []abi.PaddedPieceSize{256, 128},
			spec:        nil,
			expectedErr: errDealsUnOrdered,
		},

		// Ohter Valid Cases
		{
			name:              "128 + 512 + 512",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 512, 512},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal, isDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 512, 512},
			expectedErr:       nil,
		},

		{
			name:              "128*3 + 512",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		// Edge Cases
		{
			name:              "piece too big",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{512 << 30},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		{
			name:              "space not enough",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 1024, 1024},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal, nonDeal, isDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal limit",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MaxPiece: 2},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal size limit",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MaxPieceSize: 256},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},
	}

	for ci := range cases {
		c := cases[ci]
		expectedPieceCount := len(c.expectedPieceSize)
		require.Lenf(t, c.expectedDealIDs, expectedPieceCount, "<%s> expected deal ids & piece sizes should be equal", c.name)

		caseDeals := generateTestingDeals(c.sizes)
		gotDeals, gotErr := pickAndAlign(caseDeals, c.sectorSize, c.spec)

		if c.expectedErr != nil {
			require.ErrorIsf(t, gotErr, c.expectedErr, "<%s> expected a specified error", c.name)
			continue
		}

		require.NoErrorf(t, gotErr, "<%s> case should be valid", c.name)
		require.Lenf(t, gotDeals, expectedPieceCount, "<%s> result deals count", c.name)

		for i := range gotDeals {
			require.Equalf(t, c.expectedDealIDs[i], gotDeals[i].DealID, "<%s> id of deal %d not match", c.name, i)
			require.Equalf(t, c.expectedPieceSize[i], gotDeals[i].PieceSize, "<%s> piece size of deal %d not match", c.name, i)
		}
	}
}
