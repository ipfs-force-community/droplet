package clients

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/ipfs/go-cid"
	"go.uber.org/fx"
)

type IStorageMiner interface {
	IsUnsealed(ctx context.Context, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error)
	// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
	SectorsUnsealPiece(ctx context.Context, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid, dest string) error
}

var _ IStorageMiner = (*StorageMinerStruct)(nil)

type StorageMinerStruct struct {
	Internal struct {
		IsUnsealed func(ctx context.Context, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error)
		// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
		SectorsUnsealPiece func(ctx context.Context, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid) error
	}
}

func (s *StorageMinerStruct) IsUnsealed(ctx context.Context, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error) {
	return s.Internal.IsUnsealed(ctx, sector, offset, size)
}

func (s *StorageMinerStruct) SectorsUnsealPiece(ctx context.Context, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid, dest string) error {
	return s.Internal.SectorsUnsealPiece(ctx, sector, offset, size, randomness, commd)
}

func NewStorageMiner(mctx metrics.MetricsCtx, lc fx.Lifecycle, cfg *config.Sealer) (IStorageMiner, error) {
	apiInfo := apiinfo.NewAPIInfo(cfg.Url, cfg.Token)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, err
	}

	client := &StorageMinerStruct{}
	closer, err := jsonrpc.NewMergeClient(mctx, addr, "Filecoin", []interface{}{&client.Internal}, apiInfo.AuthHeader())

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return client, err
}
