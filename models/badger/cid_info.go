package badger

import (
	"context"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/v2/models/badger/statestore"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/ipfs/go-cid"
)

func NewBadgerCidInfoRepo(cidInfoDs CIDInfoDS) repo.ICidInfoRepo {
	return &badgerCidInfoRepo{cidInfos: statestore.New(cidInfoDs)}
}

type badgerCidInfoRepo struct {
	cidInfos *statestore.StateStore
}

var _ repo.ICidInfoRepo = (*badgerCidInfoRepo)(nil)

// Store the map of blockLocations in the dealAssigner's CIDInfo store, with key `pieceCID`
func (ps *badgerCidInfoRepo) AddPieceBlockLocations(ctx context.Context, pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	for c, blockLocation := range blockLocations {
		err := ps.mutateCIDInfo(ctx, c, func(ci *piecestore.CIDInfo) error {
			for _, pbl := range ci.PieceBlockLocations {
				if pbl.PieceCID.Equals(pieceCID) && pbl.BlockLocation == blockLocation {
					return nil
				}
			}
			ci.PieceBlockLocations = append(ci.PieceBlockLocations, piecestore.PieceBlockLocation{BlockLocation: blockLocation, PieceCID: pieceCID})
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ps *badgerCidInfoRepo) ListCidInfoKeys(ctx context.Context) ([]cid.Cid, error) {
	var cis []piecestore.CIDInfo
	if err := ps.cidInfos.List(ctx, &cis); err != nil {
		return nil, err
	}

	out := make([]cid.Cid, 0, len(cis))
	for _, ci := range cis {
		out = append(out, ci.CID)
	}

	return out, nil
}

// Retrieve the CIDInfo associated with `pieceCID` from the CID info store.
func (ps *badgerCidInfoRepo) GetCIDInfo(ctx context.Context, payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	var out piecestore.CIDInfo
	if err := ps.cidInfos.Get(payloadCID).Get(ctx, &out); err != nil {
		return piecestore.CIDInfo{}, err
	}
	return out, nil
}

func (ps *badgerCidInfoRepo) ensureCIDInfo(ctx context.Context, c cid.Cid) error {
	has, err := ps.cidInfos.Has(ctx, c)

	if err != nil {
		return err
	}

	if has {
		return nil
	}

	cidInfo := piecestore.CIDInfo{CID: c}
	return ps.cidInfos.Save(ctx, c, &cidInfo)
}

func (ps *badgerCidInfoRepo) mutateCIDInfo(ctx context.Context, c cid.Cid, mutator interface{}) error {
	err := ps.ensureCIDInfo(ctx, c)
	if err != nil {
		return err
	}

	return ps.cidInfos.Get(c).Mutate(ctx, mutator)
}
