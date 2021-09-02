package clients

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/ipfs/go-cid"
	"go.uber.org/fx"
	"io"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus-market/types"
)

type IStorageMiner interface {
	IsUnsealed(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error)
	Reader(context.Context, storage.SectorRef, abi.PaddedPieceSize, abi.PaddedPieceSize) (io.ReadCloser, error)
	SectorsStatus(ctx context.Context, sid abi.SectorNumber, showOnChainInfo bool) (types.SectorInfo, error)
	// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
	SectorsUnsealPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid) error
}

var _ IStorageMiner = (*StorageMinerStruct)(nil)

type StorageMinerStruct struct {
	Internal struct {
		IsUnsealed    func(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error)
		Reader        func(context.Context, storage.SectorRef, abi.PaddedPieceSize, abi.PaddedPieceSize) (io.ReadCloser, error)
		SectorsStatus func(ctx context.Context, sid abi.SectorNumber, showOnChainInfo bool) (types.SectorInfo, error)
		// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
		SectorsUnsealPiece func(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid) error
	}
}

func (s *StorageMinerStruct) IsUnsealed(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error) {
	return s.Internal.IsUnsealed(ctx, sector, offset, size)
}

func (s *StorageMinerStruct) Reader(ctx context.Context, ref storage.SectorRef, size abi.PaddedPieceSize, size2 abi.PaddedPieceSize) (io.ReadCloser, error) {
	return s.Internal.Reader(ctx, ref, size, size2)
}

func (s *StorageMinerStruct) SectorsStatus(ctx context.Context, sid abi.SectorNumber, showOnChainInfo bool) (types.SectorInfo, error) {
	return s.Internal.SectorsStatus(ctx, sid, showOnChainInfo)
}

func (s *StorageMinerStruct) SectorsUnsealPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid) error {
	return s.Internal.SectorsUnsealPiece(ctx, sector, offset, size, randomness, commd)
}

func NewStorageMiner(mctx metrics.MetricsCtx, lc fx.Lifecycle, cfg *config.Sealer) (IStorageMiner, error) {
	apiInfo := apiinfo.NewAPIInfo(cfg.Url, cfg.Token)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, err
	}

	client := &StorageMinerStruct{}
	closer, err := jsonrpc.NewMergeClient(mctx, addr, "Sealer", []interface{}{&client.Internal}, apiInfo.AuthHeader())

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return client, err
}
