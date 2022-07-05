package retrievalprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
)

type PieceInfo struct {
	dagstore stores.DAGStoreWrapper
	dealRepo repo.StorageDealRepo
}

// GetPieceInfoFromCid take `pieceCid` priority, then `payloadCid`
func (pinfo *PieceInfo) GetPieceInfoFromCid(ctx context.Context, payloadCID cid.Cid, piececid *cid.Cid) ([]*types.MinerDeal, error) {
	if piececid != nil && (*piececid).Defined() {
		minerDeals, err := pinfo.dealRepo.GetDealsByPieceCidAndStatus(ctx, (*piececid), storageprovider.ReadyRetrievalDealStatus...)
		if err != nil {
			return nil, err
		}
		if len(minerDeals) > 0 {
			return minerDeals, nil
		}
		return nil, fmt.Errorf("unable to find deals by pieceCid:%s, %w", piececid.String(), repo.ErrNotFound)
	}

	filter := make(map[cid.Cid]struct{})
	var allMinerDeals []*types.MinerDeal
	//First get pieces from miner storage deals
	deals, err := pinfo.dealRepo.GetDealsByDataCidAndDealStatus(ctx, address.Undef, payloadCID, []types.PieceStatus{types.Proving})
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, fmt.Errorf("failed to get deals for retrieval %s", payloadCID)
	}
	if len(deals) > 0 {
		for _, deal := range deals {
			if _, ok := filter[deal.ProposalCid]; !ok {
				allMinerDeals = append(allMinerDeals, deal)
				filter[deal.ProposalCid] = struct{}{}
			}
		}
	}
	// Get all pieces that contain the target block
	piecesWithTargetBlock, err := pinfo.dagstore.GetPiecesContainingBlock(payloadCID)
	if err != nil {
		return nil, fmt.Errorf("getting pieces for cid %s: %w", payloadCID, err)
	}

	for _, pieceWithTargetBlock := range piecesWithTargetBlock {
		minerDeals, err := pinfo.dealRepo.GetDealsByPieceCidAndStatus(ctx, pieceWithTargetBlock, storageprovider.ReadyRetrievalDealStatus...)
		if err != nil {
			return nil, err
		}
		for _, deal := range minerDeals {
			if _, ok := filter[deal.ProposalCid]; !ok {
				allMinerDeals = append(allMinerDeals, deal)
				filter[deal.ProposalCid] = struct{}{}
			}
		}
	}
	if len(allMinerDeals) > 0 {
		return allMinerDeals, nil
	}
	return nil, fmt.Errorf("unable to find ready data for payload (%s), %w", payloadCID, repo.ErrNotFound)
}
