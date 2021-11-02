package retrievaladapter

import (
	"context"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

type PieceInfo struct {
	cidInfoRepo repo.ICidInfoRepo
	dealRepo    repo.StorageDealRepo
}

func (pinfo *PieceInfo) GetPieceInfoFromCid(ctx context.Context, payloadCID, pieceCID cid.Cid) (piecestore.PieceInfo, bool, error) {
	cidInfo, err := pinfo.cidInfoRepo.GetCIDInfo(payloadCID)
	if err != nil {
		return piecestore.PieceInfoUndefined, false, xerrors.Errorf("get cid info: %w", err)
	}
	var lastErr error
	for _, pieceBlockLocation := range cidInfo.PieceBlockLocations {
		pieceInfo, err := pinfo.dealRepo.GetPieceInfo(pieceBlockLocation.PieceCID)
		if err != nil {
			lastErr = err
			continue
		}

		// if client wants to retrieve the payload from a specific piece, just return that piece.
		if pieceCID.Defined() && pieceInfo.PieceCID.Equals(pieceCID) {
			return *pieceInfo, true, nil
		}

		// if client dosen't have a preference for a particular piece, prefer a piece
		// for which an unsealed sector exists.
		if pieceCID.Equals(cid.Undef) {
			return *pieceInfo, true, nil
		}

	}

	if lastErr == nil {
		lastErr = xerrors.Errorf("unknown pieceCID %s", pieceCID.String())
	}

	return piecestore.PieceInfoUndefined, false, xerrors.Errorf("could not locate piece: %w", lastErr)
}
