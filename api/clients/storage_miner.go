package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
	types2 "github.com/ipfs-force-community/venus-common-utils/types"
	"github.com/ipfs/go-cid"
)

type MarketRequestEvent interface {
	IsUnsealed(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types2.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error)
	// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
	SectorsUnsealPiece(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types2.PaddedByteIndex, size abi.PaddedPieceSize, dest string) error
}
