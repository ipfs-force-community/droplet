package storageprovider

import (
	"testing"

	"github.com/filecoin-project/go-state-types/builtin/v8/market"

	"github.com/filecoin-project/go-state-types/abi"
	mtypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/require"
)

const (
	isDeal  = abi.DealID(1)
	nonDeal = abi.DealID(0)
)

func generateTestingDeals(sizes []abi.PaddedPieceSize, lifetimes [][2]abi.ChainEpoch) []*mtypes.DealInfoIncludePath {
	deals := make([]*mtypes.DealInfoIncludePath, len(sizes))
	for si, size := range sizes {
		proposal := market.DealProposal{
			PieceSize: size,
		}

		if len(lifetimes) > 0 {
			proposal.StartEpoch = lifetimes[si][0]
			proposal.EndEpoch = lifetimes[si][1]
		}

		deals[si] = &mtypes.DealInfoIncludePath{
			DealID:       isDeal,
			DealProposal: proposal,
		}
	}

	return deals
}

func TestDealAssignPickAndAlign(t *testing.T) {
	const SectorSize2K = abi.SectorSize(2 << 10)

	cases := []struct {
		name              string
		sectorSize        abi.SectorSize
		lifetimes         [][2]abi.ChainEpoch
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
			name:              "deal count max limit",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MaxPiece: 2},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal size max limit",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MaxPieceSize: 256},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal size min limit, all good",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MinPieceSize: 128},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal size min limit 256",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MinPieceSize: 256},
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{512, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal min limit 4",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MinPiece: 4},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "deal min limit 5, empty",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 128, 128, 512},
			spec:              &mtypes.GetDealSpec{MinPiece: 5},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		{
			name:              "space min limit 128",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 256},
			spec:              nil,
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, isDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:              "space min limit 512, empty",
			sectorSize:        SectorSize2K,
			sizes:             []abi.PaddedPieceSize{128, 256},
			spec:              &mtypes.GetDealSpec{MinUsedSpace: 512},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		// for lifetime filter
		{
			name:       "no lifetime limit",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 0,
				EndEpoch:   0,
			},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:       "with start only, all included",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 10,
				EndEpoch:   0,
			},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:       "with end only, all included",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 0,
				EndEpoch:   35,
			},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:       "with start only, edge case",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 12,
				EndEpoch:   0,
			},
			expectedDealIDs:   []abi.DealID{isDeal, nonDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:       "with end only, edge case",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 0,
				EndEpoch:   33,
			},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:       "with start only, all ignored",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 14,
				EndEpoch:   0,
			},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		{
			name:       "with end only, all ignored",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 0,
				EndEpoch:   31,
			},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		{
			name:       "with start & end, all included",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 10,
				EndEpoch:   35,
			},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, isDeal, nonDeal, isDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 128, 128, 512, 1024},
			expectedErr:       nil,
		},

		{
			name:       "with start & end, all ignored as start too late",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 14,
				EndEpoch:   35,
			},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		{
			name:       "with start & end, all ignored as end too early",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 10,
				EndEpoch:   31,
			},
			expectedDealIDs:   []abi.DealID{},
			expectedPieceSize: []abi.PaddedPieceSize{},
			expectedErr:       nil,
		},

		{
			name:       "with start & end, 2 picked",
			sectorSize: SectorSize2K,
			sizes:      []abi.PaddedPieceSize{128, 128, 128, 512},
			lifetimes: [][2]abi.ChainEpoch{
				{11, 31},
				{12, 32},
				{13, 33},
				{14, 34},
			},
			spec: &mtypes.GetDealSpec{
				StartEpoch: 11,
				EndEpoch:   34,
			},
			expectedDealIDs:   []abi.DealID{isDeal, isDeal, nonDeal, nonDeal, nonDeal},
			expectedPieceSize: []abi.PaddedPieceSize{128, 128, 256, 512, 1024},
			expectedErr:       nil,
		},
	}

	for ci := range cases {
		c := cases[ci]
		expectedPieceCount := len(c.expectedPieceSize)
		require.Lenf(t, c.expectedDealIDs, expectedPieceCount, "<%s> expected deal ids & piece sizes should be equal", c.name)

		caseDeals := generateTestingDeals(c.sizes, c.lifetimes)
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
