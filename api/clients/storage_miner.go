package clients

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

type MarketRequestEvent interface {
	IsUnsealed(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error)
	// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
	SectorsUnsealPiece(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize, dest string) error
}
