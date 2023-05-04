package client

import (
	"github.com/filecoin-project/go-address"
	shared "github.com/filecoin-project/venus/venus-shared/types"
)

type dealStat struct {
}

func newDealStat() *dealStat {
	return &dealStat{}
}

type ProviderDistribution struct {
	Provider address.Address
	Total    uint64
	Uniq     uint64
	// May be too large
	uniqPieces            map[string]uint64
	DuplicationPercentage float64
}

type ReplicaDistribution struct {
	Client                address.Address
	Total                 uint64
	DuplicationPercentage float64
	ReplicasPercentage    map[string]float64
	ReplicasDistribution  []*ProviderDistribution
}

type DealDistribution struct {
	ProvidersDistribution []*ProviderDistribution
	ReplicasDistribution  []*ReplicaDistribution
}

func (ds *dealStat) dealDistribution(deals []*shared.ClientDealProposal) *DealDistribution {
	providersDistribution := make(map[address.Address]*ProviderDistribution)
	replicasDistribution := make(map[address.Address]map[address.Address]*ProviderDistribution)
	pdTotal := make(map[address.Address]uint64)
	rdTotal := make(map[address.Address]uint64)

	for _, deal := range deals {
		provider := deal.Proposal.Provider
		pieceCID := deal.Proposal.PieceCID.String()
		pieceSize := uint64(deal.Proposal.PieceSize)
		providersDistribution[provider] = fillProviderDistribution(providersDistribution[provider], pieceCID, pieceSize, provider)
		pdTotal[provider] += pieceSize

		client := deal.Proposal.Client
		providersDistribution, ok := replicasDistribution[client]
		if !ok {
			providersDistribution = make(map[address.Address]*ProviderDistribution)
		}
		providersDistribution[provider] = fillProviderDistribution(providersDistribution[provider], pieceCID, pieceSize, provider)
		replicasDistribution[client] = providersDistribution
		rdTotal[client] += pieceSize
	}

	var pds []*ProviderDistribution
	for provider, pd := range providersDistribution {
		total := pdTotal[provider]
		pd.DuplicationPercentage = float64(total-pd.Uniq) / float64(total)
		pds = append(pds, pd)
	}

	var rds []*ReplicaDistribution
	for client, pds := range replicasDistribution {
		var uniq uint64
		uniqPiece := make(map[string]struct{})
		total := rdTotal[client]
		rd := &ReplicaDistribution{Client: client, Total: total, ReplicasPercentage: make(map[string]float64, len(pds))}

		for provider, pd := range pds {
			rd.ReplicasPercentage[provider.String()] = float64(pd.Total) / float64(total)
			pd.DuplicationPercentage = float64(pd.Total-pd.Uniq) / float64(pd.Total)

			for pieceCID, size := range pd.uniqPieces {
				if _, ok := uniqPiece[pieceCID]; !ok {
					uniqPiece[pieceCID] = struct{}{}
					uniq += size
				}
			}
		}
		rd.DuplicationPercentage = float64(total-uniq) / float64(total)

		rds = append(rds, rd)
	}

	return &DealDistribution{
		ProvidersDistribution: pds,
		ReplicasDistribution:  rds,
	}
}

func fillProviderDistribution(pd *ProviderDistribution, pieceCID string, pieceSize uint64, provider address.Address) *ProviderDistribution {
	if pd == nil {
		pd = &ProviderDistribution{
			Provider:   provider,
			uniqPieces: make(map[string]uint64),
		}
	}
	pd.Total += pieceSize
	if _, ok := pd.uniqPieces[pieceCID]; !ok {
		pd.uniqPieces[pieceCID] = pieceSize
		pd.Uniq += pieceSize
	}

	return pd
}
