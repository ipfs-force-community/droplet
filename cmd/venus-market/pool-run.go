package main

import (
	"context"

	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"
	"github.com/ipfs-force-community/venus-common-utils/metrics"

	metrics2 "github.com/ipfs/go-metrics-interface"

	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/api/impl"
	cli2 "github.com/filecoin-project/venus-market/cli"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/dagstore"
	"github.com/filecoin-project/venus-market/fundmgr"
	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/paychmgr"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/filecoin-project/venus-market/retrievalprovider"
	"github.com/filecoin-project/venus-market/rpc"
	"github.com/filecoin-project/venus-market/storageprovider"
	types2 "github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
)

var poolRunCmd = &cli.Command{
	Name:      "pool-run",
	Usage:     "Run the market daemon in pool mode",
	ArgsUsage: "[minerAddress]",
	Flags: []cli.Flag{
		NodeUrlFlag,
		NodeTokenFlag,
		AuthUrlFlag,
		AuthTokeFlag,
		MessagerUrlFlag,
		MessagerTokenFlag,
		HidenSignerTypeFlag,
		GatewayUrlFlag,
		GatewayTokenFlag,
		PieceStorageFlag,
		MysqlDsnFlag,
		MinerListFlag,
		PaymentAddressFlag,
	},
	Action: poolDaemon,
}

func poolDaemon(cctx *cli.Context) error {
	utils.SetupLogLevels()
	ctx := cctx.Context
	if !cctx.IsSet(HidenSignerTypeFlag.Name) {
		if err := cctx.Set(HidenSignerTypeFlag.Name, "gateway"); err != nil {
			return xerrors.Errorf("set %s with gateway failed %v", HidenSignerTypeFlag.Name, err)
		}
	}
	cfg, err := prepare(cctx)
	if err != nil {
		return err
	}

	resAPI := &impl.MarketNodeImpl{}
	shutdownChan := make(chan struct{})
	_, err = builder.New(ctx,
		//defaults
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), func(lc fx.Lifecycle, home config.IHome, disabled journal.DisabledEvents) (journal.Journal, error) {
			return journal.OpenFilesystemJournal(lc, home.MustHomePath(), "venus-market", disabled)
		}),

		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "venus-market")
		}),
		builder.Override(new(types2.ShutdownChan), shutdownChan),
		//config
		config.ConfigServerOpts(cfg),

		// miner manager
		minermgr.MinerMgrOpts(cfg),

		//clients
		clients.ClientsOpts(true, "pool", &cfg.Messager, &cfg.Signer),
		models.DBOptions(true, &cfg.Mysql),
		network.NetworkOpts(true, cfg.SimultaneousTransfersForRetrieval, cfg.SimultaneousTransfersForStoragePerClient, cfg.SimultaneousTransfersForStorage),
		piecestorage.PieceStorageOpts(cfg),
		fundmgr.FundMgrOpts,
		dagstore.DagstoreOpts,
		paychmgr.PaychOpts,
		// Markets
		storageprovider.StorageProviderOpts(cfg),
		retrievalprovider.RetrievalProviderOpts(cfg),

		func(s *builder.Settings) error {
			s.Invokes[ExtractApiKey] = builder.InvokeOption{
				Priority: 10,
				Option:   fx.Populate(resAPI),
			}
			return nil
		},
	)
	if err != nil {
		return xerrors.Errorf("initializing node: %w", err)
	}
	finishCh := utils.MonitorShutdown(shutdownChan)

	mux := mux.NewRouter()
	if err = mux.Handle("/resource", rpc.NewPieceStorageServer(resAPI.PieceStorage)).GetError(); err != nil {
		return xerrors.Errorf("handle 'resource' failed: %w", err)
	}

	return rpc.ServeRPC(ctx, cfg, &cfg.API, mux, 1000, cli2.API_NAMESPACE_VENUS_MARKET, cfg.AuthNode.Url, api.MarketFullNode(resAPI), finishCh)
}
