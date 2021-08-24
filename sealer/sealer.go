package sealer

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var log = logging.Logger("sealer")

type SectorBuilder interface {
	SectorsStatus(ctx context.Context, sid abi.SectorNumber, showOnChainInfo bool) (types.SectorInfo, error)
}

type Unsealer interface {
	// SectorsUnsealPiece will Unseal a Sealed sector file for the given sector.
	SectorsUnsealPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, randomness abi.SealRandomness, commd *cid.Cid) error
}

type MinerStorageService interface {
	Unsealer
	SectorBuilder
}

func connectMinerService(sealerCfg *config.Sealer) func(mctx helpers.MetricsCtx, lc fx.Lifecycle) (clients.IStorageMiner, error) {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle) (clients.IStorageMiner, error) {
		ctx := helpers.LifecycleCtx(mctx, lc)
		info := apiinfo.NewAPIInfo(sealerCfg.Url, sealerCfg.Token)
		addr, err := info.DialArgs("v0")
		if err != nil {
			return nil, xerrors.Errorf("could not get DialArgs: %w", err)
		}

		log.Infof("Checking (svc) api version of %s", addr)

		mapi, closer, err := clients.NewStorageMiner(ctx, addr)
		if err != nil {
			return nil, err
		}
		lc.Append(fx.Hook{
			OnStop: func(context.Context) error {
				closer()
				return nil
			}})

		return mapi, nil
	}
}

func ConnectStorageService(mctx helpers.MetricsCtx, lc fx.Lifecycle, cfg *config.MarketConfig) (MinerStorageService, error) {
	log.Info("Connecting piecestorage service to miner")
	return connectMinerService(&cfg.Sealer)(mctx, lc)
}
