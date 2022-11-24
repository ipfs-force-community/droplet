package main

import (
	"fmt"

	"go.uber.org/fx"

	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"

	"github.com/filecoin-project/venus-market/v2/api/clients"
	"github.com/filecoin-project/venus-market/v2/api/impl"
	cli2 "github.com/filecoin-project/venus-market/v2/cli"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/dagstore"
	"github.com/filecoin-project/venus-market/v2/fundmgr"
	"github.com/filecoin-project/venus-market/v2/metrics"
	"github.com/filecoin-project/venus-market/v2/minermgr"
	"github.com/filecoin-project/venus-market/v2/models"
	"github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/paychmgr"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	"github.com/filecoin-project/venus-market/v2/retrievalprovider"
	"github.com/filecoin-project/venus-market/v2/rpc"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	types2 "github.com/filecoin-project/venus-market/v2/types"
	"github.com/filecoin-project/venus-market/v2/utils"

	"github.com/filecoin-project/venus-auth/jwtclient"

	marketapi "github.com/filecoin-project/venus/venus-shared/api/market"
	"github.com/filecoin-project/venus/venus-shared/api/permission"
)

var soloRunCmd = &cli.Command{
	Name:  "solo-run",
	Usage: "Run the market daemon in solo mode",
	Flags: []cli.Flag{
		NodeUrlFlag,
		NodeTokenFlag,
		SignerTypeFlag,
		SignerUrlFlag,
		SignerTokenFlag,
		MysqlDsnFlag,
	},
	Action: soloDaemon,
}

func soloDaemon(cctx *cli.Context) error {
	utils.SetupLogLevels()

	cfg, err := prepare(cctx, config.SignerTypeWallet)
	if err != nil {
		return fmt.Errorf("prepare solo run failed: %w", err)
	}

	// Configuration sanity check
	if len(cfg.AuthNode.Url) > 0 {
		return fmt.Errorf("solo mode does not need to configure auth node")
	}

	if len(cfg.Signer.Url) == 0 {
		return fmt.Errorf("the signer node must be configured")
	}

	ctx := cctx.Context

	resAPI := &impl.MarketNodeImpl{}
	shutdownChan := make(chan struct{})
	closeFunc, err := builder.New(ctx,
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), func(lc fx.Lifecycle, home config.IHome, disabled journal.DisabledEvents) (journal.Journal, error) {
			return journal.OpenFilesystemJournal(lc, home.MustHomePath(), "venus-market", disabled)
		}),

		metrics.MetricsOpts("venus-market", &cfg.Metrics),
		builder.Override(new(types2.ShutdownChan), shutdownChan),

		// override marketconfig
		builder.Override(new(config.MarketConfig), cfg),

		// config
		config.ConfigServerOpts(cfg),

		// 'solo' mode doesn't needs a 'AuthClient' of venus-auth,
		// provide a nil 'AuthClient', just for making 'NewUserMgrImpl' happy
		builder.Override(new(*jwtclient.AuthClient), func() *jwtclient.AuthClient { return nil }),

		// user manager
		minermgr.MinerMgrOpts(),

		// clients
		clients.ClientsOpts(true, "solo", &cfg.Messager, &cfg.Signer),
		models.DBOptions(true, &cfg.Mysql),
		network.NetworkOpts(true, cfg.SimultaneousTransfersForRetrieval, cfg.SimultaneousTransfersForStoragePerClient, cfg.SimultaneousTransfersForStorage),
		piecestorage.PieceStorageOpts(&cfg.PieceStorage),
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
		return fmt.Errorf("initializing node: %w", err)
	}
	defer closeFunc(ctx) //nolint

	finishCh := utils.MonitorShutdown(shutdownChan)

	router := mux.NewRouter()
	router.Handle("resource", rpc.NewPieceStorageServer(resAPI.PieceStorageMgr))

	var fullAPI marketapi.IMarketStruct
	permission.PermissionProxy(marketapi.IMarket(resAPI), &fullAPI)

	return rpc.ServeRPC(ctx, cfg, &cfg.API, router, 1000, cli2.API_NAMESPACE_VENUS_MARKET, nil, &fullAPI, finishCh)
}
