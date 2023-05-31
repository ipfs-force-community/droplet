package client

import (
	"github.com/filecoin-project/go-address"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
)

type dealStat struct {
}

func newDealStat() *dealStat {
	return &dealStat{}
}

func (ds *dealStat) dealDistribution(deals []*shared.ClientDealProposal) *types.DealDistribution {
	providersDistribution := make(map[address.Address]*types.ProviderDistribution)
	replicasDistribution := make(map[address.Address]map[address.Address]*types.ProviderDistribution)
	rdTotal := make(map[address.Address]uint64)

	for _, deal := range deals {
		provider := deal.Proposal.Provider
		pieceCID := deal.Proposal.PieceCID.String()
		pieceSize := uint64(deal.Proposal.PieceSize)
		providersDistribution[provider] = fillProviderDistribution(providersDistribution[provider], pieceCID, pieceSize, provider)

		client := deal.Proposal.Client
		tmp, ok := replicasDistribution[client]
		if !ok {
			tmp = make(map[address.Address]*types.ProviderDistribution)
		}
		tmp[provider] = fillProviderDistribution(tmp[provider], pieceCID, pieceSize, provider)
		replicasDistribution[client] = tmp
		rdTotal[client] += pieceSize
	}

	var pds []*types.ProviderDistribution
	for _, pd := range providersDistribution {
		pd.DuplicationPercentage = float64(pd.Total-pd.Uniq) / float64(pd.Total)
		pds = append(pds, pd)
	}

	var rds []*types.ReplicaDistribution
	for client, pds := range replicasDistribution {
		var uniq uint64
		uniqPiece := make(map[string]struct{})
		total := rdTotal[client]
		rd := &types.ReplicaDistribution{Client: client, Total: total, ReplicasPercentage: make(map[string]float64, len(pds))}

		for provider, pd := range pds {
			rd.ReplicasPercentage[provider.String()] = float64(pd.Total) / float64(total)
			pd.DuplicationPercentage = float64(pd.Total-pd.Uniq) / float64(pd.Total)

			for pieceCID, size := range pd.UniqPieces {
				if _, ok := uniqPiece[pieceCID]; !ok {
					uniqPiece[pieceCID] = struct{}{}
					uniq += size
				}
			}
			rd.ReplicasDistribution = append(rd.ReplicasDistribution, pd)
		}
		rd.Uniq = uniq
		rd.DuplicationPercentage = float64(rd.Total-rd.Uniq) / float64(rd.Total)
		rds = append(rds, rd)
	}

	return &types.DealDistribution{
		ProvidersDistribution: pds,
		ReplicasDistribution:  rds,
	}
}

func fillProviderDistribution(pd *types.ProviderDistribution, pieceCID string, pieceSize uint64, provider address.Address) *types.ProviderDistribution {
	if pd == nil {
		pd = &types.ProviderDistribution{
			Provider:   provider,
			UniqPieces: make(map[string]uint64),
		}
	}
	pd.Total += pieceSize
	if _, ok := pd.UniqPieces[pieceCID]; !ok {
		pd.UniqPieces[pieceCID] = pieceSize
		pd.Uniq += pieceSize
	}

	return pd
}
