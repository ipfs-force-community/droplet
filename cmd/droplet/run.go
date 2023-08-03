package main

import (
	"fmt"
	"os"

	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"

	"github.com/ipfs-force-community/sophon-auth/jwtclient"

	"github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/api/impl"
	"github.com/ipfs-force-community/droplet/v2/api/impl/v0api"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/cmd"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/dagstore"
	"github.com/ipfs-force-community/droplet/v2/fundmgr"
	"github.com/ipfs-force-community/droplet/v2/metrics"
	"github.com/ipfs-force-community/droplet/v2/minermgr"
	"github.com/ipfs-force-community/droplet/v2/models"
	"github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/paychmgr"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs-force-community/droplet/v2/retrievalprovider"
	"github.com/ipfs-force-community/droplet/v2/retrievalprovider/httpretrieval"
	"github.com/ipfs-force-community/droplet/v2/rpc"
	"github.com/ipfs-force-community/droplet/v2/storageprovider"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
	"github.com/ipfs-force-community/droplet/v2/utils"

	marketapiV1 "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/api/permission"
)

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "Run the market daemon",
	Flags: []cli.Flag{
		APIListenFlag,
		ChainServiceTokenFlag,
		ChainServiceUrlFlag,
		NodeUrlFlag,
		AuthUrlFlag,
		MessagerUrlFlag,
		GatewayUrlFlag,
		SignerTypeFlag,
		SignerUrlFlag,
		SignerTokenFlag,
		MysqlDsnFlag,
	},
	Action: runDaemon,
}

func flagData(cctx *cli.Context, cfg *config.MarketConfig) error {
	if cctx.IsSet(APIListenFlag.Name) {
		cfg.API.ListenAddress = cctx.String(APIListenFlag.Name)
	}

	if cctx.IsSet(NodeUrlFlag.Name) {
		if cfg.Node == nil {
			cfg.Node = &config.Node{}
		}

		cfg.Node.Url = cctx.String(NodeUrlFlag.Name)
	}

	if cctx.IsSet(AuthUrlFlag.Name) {
		if cfg.AuthNode == nil {
			cfg.AuthNode = &config.AuthNode{}
		}
		cfg.AuthNode.Url = cctx.String(AuthUrlFlag.Name)
	}

	if cctx.IsSet(MessagerUrlFlag.Name) {
		if cfg.Messager == nil {
			cfg.Messager = &config.Messager{}
		}
		cfg.Messager.Url = cctx.String(MessagerUrlFlag.Name)
	}

	// chain service token
	if cctx.IsSet(ChainServiceTokenFlag.Name) {
		if cfg.ChainService == nil {
			cfg.ChainService = &config.ChainService{}
		}
		csToken := cctx.String(ChainServiceTokenFlag.Name)
		cfg.ChainService.Token = csToken
	}

	if cctx.IsSet(ChainServiceUrlFlag.Name) {
		if cfg.ChainService == nil {
			cfg.ChainService = &config.ChainService{}
		}
		cfg.ChainService.Url = cctx.String(ChainServiceUrlFlag.Name)
	}

	if cctx.IsSet(SignerTypeFlag.Name) {
		signerType := cctx.String(SignerTypeFlag.Name)
		switch signerType {
		case config.SignerTypeGateway:
			if cctx.IsSet(GatewayUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(GatewayUrlFlag.Name)
			}

			if cctx.IsSet(ChainServiceTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(ChainServiceTokenFlag.Name)
			}
		case config.SignerTypeWallet:
			if cctx.IsSet(SignerUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(SignerUrlFlag.Name)
			}

			if cctx.IsSet(SignerTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(SignerTokenFlag.Name)
			}
		case config.SignerTypeLotusnode:
			if cctx.IsSet(NodeUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(NodeUrlFlag.Name)
			}

			if cctx.IsSet(ChainServiceTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(ChainServiceTokenFlag.Name)
				if cfg.AuthNode == nil {
					cfg.AuthNode = &config.AuthNode{}
				}
				cfg.AuthNode.Token = cctx.String(ChainServiceTokenFlag.Name)
			}
		default:
			return fmt.Errorf("unsupport signer type %s", signerType)
		}
		cfg.Signer.SignerType = signerType
	}

	if cfg.Signer.SignerType == config.SignerTypeLotusnode {
		cfg.Messager.Token = ""
		cfg.AuthNode.Token = ""
	}

	if cctx.IsSet(MysqlDsnFlag.Name) {
		cfg.Mysql.ConnectionString = cctx.String(MysqlDsnFlag.Name)
	}

	return nil
}

func prepare(cctx *cli.Context) (*config.MarketConfig, error) {
	var err error
	cfg := config.DefaultMarketConfig
	cfg.HomeDir, err = cli2.GetRepoPath(cctx, RepoFlag.Name, cli2.OldMarketRepoPath)
	if err != nil {
		return nil, err
	}
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}

	mainLog.Info("load config from path ", cfgPath)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//create
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag: %w", err)
		}

		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("save config to %s: %w", cfgPath, err)
		}
	} else if err == nil {
		//loadConfig
		err = config.LoadConfig(cfgPath, cfg)
		if err != nil {
			return nil, err
		}

		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag: %w", err)
		}
	} else {
		return nil, err
	}

	return cfg, cmd.FetchAndLoadBundles(cctx.Context, *cfg.GetNode())
}

func runDaemon(cctx *cli.Context) error {
	utils.SetupLogLevels()

	cfg, err := prepare(cctx)
	if err != nil {
		return fmt.Errorf("prepare run failed: %w", err)
	}

	if len(cfg.Signer.Url) == 0 {
		return fmt.Errorf("the signer node must be configured")
	}

	ctx := cctx.Context

	// 'NewAuthClient' never returns an error, no needs to check
	var authClient *jwtclient.AuthClient
	authNode := cfg.GetAuthNode()
	if len(authNode.Url) != 0 {
		if len(authNode.Token) == 0 {
			return fmt.Errorf("the auth node token must be configured if auth node url is configured")
		}
		authClient, _ = jwtclient.NewAuthClient(authNode.Url, authNode.Token)
	}

	var iAuthClient jwtclient.IAuthClient = authClient
	if authClient == nil {
		iAuthClient = &models.IAuthClientStub{}
	}

	resAPI := &impl.MarketNodeImpl{}
	shutdownChan := make(chan struct{})
	closeFunc, err := builder.New(ctx,
		// defaults
		builder.Override(new(jwtclient.IAuthClient), iAuthClient),
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), func(lc fx.Lifecycle, home config.IHome, disabled journal.DisabledEvents) (journal.Journal, error) {
			return journal.OpenFilesystemJournal(lc, home.MustHomePath(), "droplet", disabled)
		}),

		metrics.MetricsOpts("droplet", &cfg.Metrics),
		// override marketconfig
		builder.Override(new(config.MarketConfig), cfg),
		builder.Override(new(types2.ShutdownChan), shutdownChan),

		//config
		config.ConfigServerOpts(cfg),

		// user manager
		minermgr.MinerMgrOpts(),

		// clients
		clients.ClientsOpts(true, cfg.GetMessager(), &cfg.Signer, authClient),
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
	if err = router.Handle("/resource", rpc.NewPieceStorageServer(resAPI.PieceStorageMgr)).GetError(); err != nil {
		return fmt.Errorf("handle 'resource' failed: %w", err)
	}
	httpRetrievalServer := httpretrieval.NewServer(resAPI.PieceStorageMgr)

	var iMarket marketapiV1.IMarketStruct
	permission.PermissionProxy(marketapiV1.IMarket(resAPI), &iMarket)

	api := (marketapiV1.IMarket)(&iMarket)
	apiHandles := []rpc.APIHandle{
		{Path: "/rpc/v1", API: api},
		{Path: "/rpc/v0", API: v0api.WrapperV1IMarket{IMarket: api}},
	}

	return rpc.ServeRPC(ctx, cfg, &cfg.API, router, 1000, cli2.API_NAMESPACE_VENUS_MARKET, authClient, apiHandles, finishCh, httpRetrievalServer)
}
