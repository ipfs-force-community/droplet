package client

import (
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
	clients := make([]address.Address, 5)
	pieceSize := abi.PaddedPieceSize(10000)

	testutil.Provide(t, &pieceCIDs)
	testutil.Provide(t, &deals)
	testutil.Provide(t, &providers)
	testutil.Provide(t, &clients)

	var verifiedDeal []*shared.ClientDealProposal
	expectProvidersDistribution := make(map[address.Address]*types.ProviderDistribution)
	expectReplicasDistribution := make(map[address.Address]*types.ReplicaDistribution)
	for i := 0; i < len(deals); i++ {
		deal := deals[i]
		deal.Proposal.PieceCID = pieceCIDs[i%len(pieceCIDs)]
		deal.Proposal.PieceSize = pieceSize
		deal.Proposal.Client = clients[i%len(clients)]
		deal.Proposal.Provider = providers[i%len(providers)]
		if i%2 == 0 {
			deal.Proposal.VerifiedDeal = true
			verifiedDeal = append(verifiedDeal, deal)
			if i < len(deals)/2 {
				pd := &types.ProviderDistribution{
					Provider:              deal.Proposal.Provider,
					Total:                 uint64(deal.Proposal.PieceSize) * 2,
					Uniq:                  uint64(deal.Proposal.PieceSize),
					DuplicationPercentage: 0.5,
					UniqPieces: map[string]uint64{
						deal.Proposal.PieceCID.String(): uint64(pieceSize),
					},
				}
				expectProvidersDistribution[pd.Provider] = pd
			}
		}
	}

	for i := range []int{0, 1, 2, 3, 4} {
		provider := verifiedDeal[i].Proposal.Provider
		if i == 1 || i == 3 {
			provider = verifiedDeal[i+5].Proposal.Provider
		}
		pd := expectProvidersDistribution[provider]
		rd := &types.ReplicaDistribution{
			Client:                verifiedDeal[i].Proposal.Client,
			Total:                 uint64(verifiedDeal[i].Proposal.PieceSize) * 2,
			Uniq:                  uint64(verifiedDeal[i].Proposal.PieceSize) * 1,
			DuplicationPercentage: 0.5,
			ReplicasPercentage: map[string]float64{
				provider.String(): 1,
			},
			ReplicasDistribution: []*types.ProviderDistribution{
				pd,
			},
		}
		expectReplicasDistribution[rd.Client] = rd
	}

	dealStat := newDealStat()
	dd := dealStat.dealDistribution(verifiedDeal)

	assert.Len(t, dd.ProvidersDistribution, 5)
	for _, pd := range dd.ProvidersDistribution {
		expect := expectProvidersDistribution[pd.Provider]
		assert.NotNil(t, expect)
		assert.Equal(t, expect, pd)
	}

	assert.Len(t, dd.ReplicasDistribution, 5)
	for _, rd := range dd.ReplicasDistribution {
		expect := expectReplicasDistribution[rd.Client]
		assert.NotNil(t, expect)
		assert.Equal(t, expect, rd)
	}
}
