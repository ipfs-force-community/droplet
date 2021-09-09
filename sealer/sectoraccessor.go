package sealer

import (
	"context"
	"github.com/filecoin-project/venus-market/clients"
	types2 "github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/miner"
	"io"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
	specstorage "github.com/filecoin-project/specs-storage/storage"
	"github.com/ipfs/go-cid"
)

type sectorAccessor struct {
	maddr    address.Address
	minerapi clients.IStorageMiner
	pp       PieceProvider
	full     apiface.FullNode
}

var _ retrievalmarket.SectorAccessor = (*sectorAccessor)(nil)

func NewSectorAccessor(maddr types2.MinerAddress, minerapi clients.IStorageMiner, pp PieceProvider, full apiface.FullNode) retrievalmarket.SectorAccessor {
	return &sectorAccessor{address.Address(maddr), minerapi, pp, full}
}

func (sa *sectorAccessor) UnsealSector(ctx context.Context, sectorID abi.SectorNumber, offset abi.UnpaddedPieceSize, length abi.UnpaddedPieceSize) (io.ReadCloser, error) {
	log.Debugf("get sector %d, offset %d, length %d", sectorID, offset, length)
	mid, err := address.IDFromAddress(sa.maddr)
	if err != nil {
		return nil, err
	}

	spt, err := sa.getSealProofType(ctx)
	if err != nil {
		return nil, err
	}
	ref := specstorage.SectorRef{
		ID: abi.SectorID{
			Miner:  abi.ActorID(mid),
			Number: sectorID,
		},
		ProofType: spt,
	}

	// Get a reader for the piece, unsealing the piece if necessary
	log.Debugf("read piece in sector %d, offset %d, length %d from miner %d", sectorID, offset, length, mid)
	r, unsealed, err := sa.pp.ReadPiece(ctx, ref, storiface.UnpaddedByteIndex(offset), length, nil, cid.Undef)
	if err != nil {
		return nil, xerrors.Errorf("failed to unseal piece from sector %d: %w", sectorID, err)
	}
	_ = unsealed // todo: use

	return r, nil
}

func (sa *sectorAccessor) IsUnsealed(ctx context.Context, sectorID abi.SectorNumber, offset abi.UnpaddedPieceSize, length abi.UnpaddedPieceSize) (bool, error) {
	mid, err := address.IDFromAddress(sa.maddr)
	if err != nil {
		return false, err
	}

	spt, err := sa.getSealProofType(ctx)
	if err != nil {
		return false, err
	}

	ref := specstorage.SectorRef{
		ID: abi.SectorID{
			Miner:  abi.ActorID(mid),
			Number: sectorID,
		},
		ProofType: spt,
	}

	log.Debugf("will call IsUnsealed now sector=%+v, offset=%d, size=%d", sectorID, offset, length)
	return sa.pp.IsUnsealed(ctx, ref, storiface.UnpaddedByteIndex(offset), length)
}

func (sa *sectorAccessor) getSealProofType(ctx context.Context) (abi.RegisteredSealProof, error) {
	mi, err := sa.full.StateMinerInfo(ctx, sa.maddr, types.EmptyTSK)
	if err != nil {
		return 0, err
	}

	ver, err := sa.full.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return 0, err
	}

	return miner.PreferredSealProofTypeFromWindowPoStType(ver, mi.WindowPoStProofType)
}
