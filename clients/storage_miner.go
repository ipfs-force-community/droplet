package clients

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus-market/types"
)

type IStorageMiner interface {
	SectorsStatus(ctx context.Context, sid abi.SectorNumber, showOnChainInfo bool) (types.SectorInfo, error)
	// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
	SectorsUnsealPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid) error
}

func NewStorageMiner(ctx context.Context, apiInfo string) (IStorageMiner, jsonrpc.ClientCloser, error) {
	panic("to impl")
}
