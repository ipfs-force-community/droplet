package client

import (
	"sort"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestStat(t *testing.T) {
	pieceCIDs := make([]cid.Cid, 10)
	deals := make([]*shared.ClientDealProposal, 20)
	providers := make([]address.Address, 10)
	clients := make([]address.Address, 4)
	pieceSize := abi.PaddedPieceSize(10000)

	testutil.Provide(t, &pieceCIDs)
	testutil.Provide(t, &deals)
	testutil.Provide(t, &providers)
	testutil.Provide(t, &clients)

	var expect types.DealDistribution
	var verifiedDeal []*shared.ClientDealProposal
	for i := 0; i < len(deals); i++ {
		deal := deals[i]
		deal.Proposal.PieceCID = pieceCIDs[i%len(pieceCIDs)]
		deal.Proposal.PieceSize = pieceSize
		deal.Proposal.Client = clients[i%len(clients)]
		deal.Proposal.Provider = providers[i%len(providers)]
		deal.Proposal.VerifiedDeal = false
		if i%2 == 0 {
			deal.Proposal.VerifiedDeal = true
		}

		if deal.Proposal.VerifiedDeal {
			verifiedDeal = append(verifiedDeal, deal)

			var found bool
			for _, pd := range expect.ProvidersDistribution {
				if pd.Provider == deal.Proposal.Provider {
					found = true
					break
				}
			}

			pd := &types.ProviderDistribution{
				Provider: deal.Proposal.Provider,
				Total:    uint64(deal.Proposal.PieceSize) * 2,
				Uniq:     uint64(deal.Proposal.PieceSize),
				UniqPieces: map[string]uint64{
					deal.Proposal.PieceCID.String(): uint64(pieceSize),
				},
			}
			pd.DuplicationPercentage = float64(pd.Uniq) / float64(pd.Total)
			if !found {
				expect.ProvidersDistribution = append(expect.ProvidersDistribution, pd)
			}

			found = false
			for _, rd := range expect.ReplicasDistribution {
				if rd.Client == deal.Proposal.Client {
					found = true
					break
				}
			}

			if !found {
				rd := &types.ReplicaDistribution{
					Client:                deal.Proposal.Client,
					Total:                 uint64(len(deals) / len(clients) * int(pieceSize)),
					DuplicationPercentage: 0,
					ReplicasPercentage:    map[string]float64{},
					ReplicasDistribution:  []*types.ProviderDistribution{pd},
				}
				rd.ReplicasPercentage[deal.Proposal.Provider.String()] = float64(pieceSize) / float64(rd.Total)
				expect.ReplicasDistribution = append(expect.ReplicasDistribution, rd)
			} else {
				found = false
				for _, rd := range expect.ReplicasDistribution {
					if rd.Client == deal.Proposal.Client {
						var pdTotal uint64
						for _, pd := range rd.ReplicasDistribution {
							if pd.Provider == deal.Proposal.Provider {
								pd.Total += uint64(deal.Proposal.PieceSize)
								if _, ok := pd.UniqPieces[deal.Proposal.PieceCID.String()]; !ok {
									pd.UniqPieces[deal.Proposal.PieceCID.String()] = uint64(deal.Proposal.PieceSize)
									pd.Uniq += uint64(deal.Proposal.PieceSize)
								}
								pd.DuplicationPercentage = float64(pd.Total-pd.Uniq) / float64(pd.Total)

								pdTotal = pd.Total
								found = true
								break
							}
						}

						if !found {
							rd.ReplicasDistribution = append(rd.ReplicasDistribution, pd)
							pdTotal = uint64(deal.Proposal.PieceSize)
						}

						rd.ReplicasPercentage[deal.Proposal.Provider.String()] = float64(pdTotal) / float64(rd.Total)
					}
				}
			}
		}
	}

	dealStat := newDealStat()
	dd := dealStat.dealDistribution(verifiedDeal)

	sorted := func(pds []*types.ProviderDistribution) {
		sort.Slice(pds, func(i, j int) bool {
			return pds[i].Provider.String() < pds[j].Provider.String()
		})
	}

	sorted(dd.ProvidersDistribution)
	sorted(expect.ProvidersDistribution)
	for i, pd := range dd.ProvidersDistribution {
		assert.Equal(t, expect.ProvidersDistribution[i], pd)
	}

	assert.Equal(t, len(expect.ReplicasDistribution), len(expect.ReplicasDistribution))
	for i := 0; i < len(dd.ReplicasDistribution); i++ {
		var erd *types.ReplicaDistribution
		ard := dd.ReplicasDistribution[i]
		for _, rd := range expect.ReplicasDistribution {
			if rd.Client == ard.Client {
				erd = rd
				break
			}
		}
		assert.NotNil(t, erd)

		for k, v := range ard.ReplicasPercentage {
			vv, ok := erd.ReplicasPercentage[k]
			assert.True(t, ok)
			assert.Equal(t, vv, v)
		}
		assert.Equal(t, erd.DuplicationPercentage, ard.DuplicationPercentage)
		assert.Equal(t, erd.Total, ard.Total)
		for _, pd := range ard.ReplicasDistribution {
			assert.Contains(t, erd.ReplicasDistribution, pd)
		}
	}
}
