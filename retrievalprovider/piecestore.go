package retrievalprovider

import (
	"context"

	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/storageprovider"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

type PieceInfo struct {
	cidInfoRepo repo.ICidInfoRepo
	dealRepo    repo.StorageDealRepo
}

func (pinfo *PieceInfo) GetPieceInfoFromCid(ctx context.Context, payloadCID cid.Cid, piececid *cid.Cid) ([]*types.MinerDeal, error) {
	cidInfo, err := pinfo.cidInfoRepo.GetCIDInfo(ctx, payloadCID)
	if err != nil {
		return nil, xerrors.Errorf("get cid info: %w", err)
	}

	if piececid != nil && (*piececid).Defined() {
		minerDeals, err := pinfo.dealRepo.GetDealsByPieceCidAndStatus(ctx, (*piececid), storageprovider.ReadyRetrievalDealStatus...)
		if err != nil {
			return nil, err
		}
		return minerDeals, nil
	}

	var allMinerDeals []*types.MinerDeal
	for _, pieceBlockLocation := range cidInfo.PieceBlockLocations {
		minerDeals, err := pinfo.dealRepo.GetDealsByPieceCidAndStatus(ctx, pieceBlockLocation.PieceCID, storageprovider.ReadyRetrievalDealStatus...)
		if err != nil {
			return nil, err
		}
		allMinerDeals = append(allMinerDeals, minerDeals...)
	}
	if len(allMinerDeals) > 0 {
		return allMinerDeals, nil
	}
	return nil, xerrors.Errorf("unable to find ready data for piece (%s) payload (%s)", piececid, payloadCID)
}
