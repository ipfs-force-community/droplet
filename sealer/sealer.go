package sealer

import (
	"context"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	"github.com/filecoin-project/lotus/storage/sectorblocks"
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var log = logging.Logger("sealer")

type MinerSealingService api.StorageMiner
type MinerStorageService api.StorageMiner

var _ sectorblocks.SectorBuilder = *new(MinerSealingService)

func connectMinerService(sealerCfg *config.Sealer) func(mctx helpers.MetricsCtx, lc fx.Lifecycle) (api.StorageMiner, error) {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle) (api.StorageMiner, error) {
		ctx := helpers.LifecycleCtx(mctx, lc)
		info := apiinfo.NewAPIInfo(sealerCfg.Url, sealerCfg.Token)
		addr, err := info.DialArgs("v0")
		if err != nil {
			return nil, xerrors.Errorf("could not get DialArgs: %w", err)
		}

		log.Infof("Checking (svc) api version of %s", addr)

		mapi, closer, err := client.NewStorageMinerRPCV0(ctx, addr, info.AuthHeader())
		if err != nil {
			return nil, err
		}
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				v, err := mapi.Version(ctx)
				if err != nil {
					return xerrors.Errorf("checking version: %w", err)
				}

				if !v.APIVersion.EqMajorMinor(api.MinerAPIVersion0) {
					return xerrors.Errorf("remote service API version didn't match (expected %s, remote %s)", api.MinerAPIVersion0, v.APIVersion)
				}

				return nil
			},
			OnStop: func(context.Context) error {
				closer()
				return nil
			}})

		return mapi, nil
	}
}

func ConnectSealingService(mctx helpers.MetricsCtx, lc fx.Lifecycle, cfg *config.Market) (MinerSealingService, error) {
	log.Info("Connecting sealing service to miner")
	return connectMinerService(&cfg.Sealer)(mctx, lc)
}

func ConnectStorageService(mctx helpers.MetricsCtx, lc fx.Lifecycle, cfg *config.Market) (MinerStorageService, error) {
	log.Info("Connecting storage service to miner")
	return connectMinerService(&cfg.Sealer)(mctx, lc)
}
