package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	metrics2 "github.com/ipfs/go-metrics-interface"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"

	clients2 "github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/api/impl"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/client"
	"github.com/ipfs-force-community/droplet/v2/cmd"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/fundmgr"
	"github.com/ipfs-force-community/droplet/v2/models"
	"github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/paychmgr"
	"github.com/ipfs-force-community/droplet/v2/rpc"
	"github.com/ipfs-force-community/droplet/v2/storageprovider"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/ipfs-force-community/droplet/v2/version"

	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"

	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"

	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	"github.com/filecoin-project/venus/venus-shared/api/permission"
)

const (
	oldRepoPath = "~/.marketclient"
	defRepoPath = "~/.droplet-client"
)

var (
	ExtractApiKey = builder.NextInvoke()
	log           = logging.Logger("main")
)

var (
	RepoFlag = &cli.StringFlag{
		Name:    "repo",
		EnvVars: []string{"DROPLET_CLIENT_PATH", "VENUS_MARKET_CLIENT_PATH"},
		Value:   defRepoPath,
	}

	APIListenFlag = &cli.StringFlag{
		Name:  "listen",
		Usage: "specify endpoint for listen",
		Value: "/ip4/127.0.0.1/tcp/41231",
	}

	NodeUrlFlag = &cli.StringFlag{
		Name:  "node-url",
		Usage: "url to connect to full node",
	}
	NodeTokenFlag = &cli.StringFlag{
		Name:  "node-token",
		Usage: "token for connect full node",
	}

	MessagerUrlFlag = &cli.StringFlag{
		Name:  "messager-url",
		Usage: "url to connect the sophon-messager service of the chain service layer",
	}

	MessagerTokenFlag = &cli.StringFlag{
		Name:  "messager-token",
		Usage: "messager token",
	}

	AuthTokenFlag = &cli.StringFlag{
		Name:  "auth-token",
		Usage: "token used to connect venus chain service components, eg. sophon-meassger, venus",
	}

	SignerTypeFlag = &cli.StringFlag{
		Name:   "signer-type",
		Usage:  "signer service type(lotusnode, wallet)",
		Value:  config.SignerTypeWallet,
		Hidden: false,
	}
	SignerUrlFlag = &cli.StringFlag{
		Name:  "signer-url",
		Usage: "used to connect wallet service for sign",
	}
	SignerTokenFlag = &cli.StringFlag{
		Name:  "signer-token",
		Usage: "wallet token for connect signer service",
	}

	DefaultAddressFlag = &cli.StringFlag{
		Name:  "addr",
		Usage: "default client address",
	}
)

func main() {
	localCommand := []*cli.Command{
		cli2.WithCategory("storage", storageCmd),
		cli2.WithCategory("retrieval", retrievalCmd),
		cli2.WithCategory("data", dataCmd),
		cli2.WithCategory("transfer", transferCmd),
		cli2.WithCategory("actor-funds", actorFundsCmd),
		cli2.WithCategory("paych", paychCmd),
		cli2.WithCategory("datacap", datacapCmds),
	}

	app := &cli.App{
		Name:                 "droplet-client",
		Usage:                "venus stores or retrieves the droplet client",
		Version:              version.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			RepoFlag,
		},
		Commands: append(
			localCommand,
			&cli.Command{
				Name: "run",
				Usage: "run droplet client daemon,(1) connect full node service: ./droplet-client run --node-url=<...> --node-token=<...> --addr=<WALLET_ADDR>;" +
					"(2) connect venus shared service: ./droplet-client run --node-url=<...> --messager-url=<...> --auth-token=<...>  --signer-url=<...> --signer-token=<...> --addr=<WALLET_ADDR>.",
				Flags: []cli.Flag{
					APIListenFlag,
					NodeUrlFlag,
					NodeTokenFlag,
					MessagerUrlFlag,
					MessagerTokenFlag,
					AuthTokenFlag,
					SignerTypeFlag,
					SignerUrlFlag,
					SignerTokenFlag,
					DefaultAddressFlag,
				},
				Action: marketClient,
			}),
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func flagData(cctx *cli.Context, cfg *config.MarketClientConfig) error {
	if cctx.IsSet(APIListenFlag.Name) {
		cfg.API.ListenAddress = cctx.String(APIListenFlag.Name)
	}

	if cctx.IsSet(NodeUrlFlag.Name) {
		cfg.Node.Url = cctx.String(NodeUrlFlag.Name)
		cfg.Signer.Url = cctx.String(NodeUrlFlag.Name)
	}

	if cctx.IsSet(MessagerUrlFlag.Name) {
		if !cctx.IsSet(AuthTokenFlag.Name) {
			return fmt.Errorf("the auth-token must be set when connecting to the venus chain service")
		}

		cfg.Messager.Url = cctx.String(MessagerUrlFlag.Name)
	}

	if cctx.IsSet(AuthTokenFlag.Name) {
		cfg.Messager.Token = cctx.String(AuthTokenFlag.Name)
		cfg.Node.Token = cctx.String(AuthTokenFlag.Name)
	}

	if cctx.IsSet(NodeTokenFlag.Name) {
		cfg.Node.Token = cctx.String(NodeTokenFlag.Name)
		cfg.Signer.Token = cctx.String(NodeTokenFlag.Name)
	}

	if cctx.IsSet(MessagerTokenFlag.Name) {
		cfg.Messager.Token = cctx.String(MessagerTokenFlag.Name)
	}

	signerType := cctx.String(SignerTypeFlag.Name)
	switch signerType {
	case config.SignerTypeWallet:
		{
			if cctx.IsSet(SignerUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(SignerUrlFlag.Name)
			}
			if cctx.IsSet(SignerTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(SignerTokenFlag.Name)
			}
		}
	case config.SignerTypeLotusnode:
		{
			if cctx.IsSet(NodeUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(NodeUrlFlag.Name)
			}
			if cctx.IsSet(NodeTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(NodeTokenFlag.Name)
			}
		}
	default:
		return fmt.Errorf("unsupport signer type %s", signerType)
	}
	cfg.Signer.SignerType = signerType

	if cctx.IsSet(DefaultAddressFlag.Name) {
		addr, err := address.NewFromString(cctx.String(DefaultAddressFlag.Name))
		if err != nil {
			return err
		}
		log.Infof("set default client address %s", addr.String())
		cfg.DefaultMarketAddress = config.Address(addr)
	}

	return nil
}

func prepare(cctx *cli.Context) (*config.MarketClientConfig, error) {
	var err error
	cfg := config.DefaultMarketClientConfig
	cfg.HomeDir, err = cmd.GetRepoPath(cctx, RepoFlag.Name, oldRepoPath)
	if err != nil {
		return nil, err
	}
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// create
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag %w", err)
		}

		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("save config to %s %w", cfgPath, err)
		}
	} else if err == nil {
		// loadConfig
		err = config.LoadConfig(cfgPath, cfg)
		if err != nil {
			return nil, err
		}
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag %w", err)
		}
	} else {
		return nil, err
	}

	return cfg, nil
}

func marketClient(cctx *cli.Context) error {
	utils.SetupLogLevels()
	ctx := cctx.Context
	cfg, err := prepare(cctx)
	if err != nil {
		return err
	}

	// Configuration sanity check
	if len(cfg.Signer.Url) == 0 {
		return fmt.Errorf("the signer node must be configured")
	}

	if err := cmd.FetchAndLoadBundles(cctx.Context, cfg.Node); err != nil {
		return err
	}

	resAPI := &impl.MarketClientNodeImpl{}
	shutdownChan := make(chan struct{})
	closeFunc, err := builder.New(ctx,
		// defaults
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), func(lc fx.Lifecycle, home config.IHome, disabled journal.DisabledEvents) (journal.Journal, error) {
			return journal.OpenFilesystemJournal(lc, home.MustHomePath(), "droplet-client", disabled)
		}),

		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "droplet")
		}),
		builder.Override(new(types2.ShutdownChan), shutdownChan),

		config.ConfigClientOpts(cfg),

		clients2.ClientsOpts(false, &cfg.Messager, &cfg.Signer, nil),
		models.DBOptions(false, nil),
		network.NetworkOpts(false, cfg.SimultaneousTransfersForStorage, 0, cfg.SimultaneousTransfersForRetrieval),
		paychmgr.PaychOpts,
		fundmgr.FundMgrOpts,
		storageprovider.StorageClientOpts,
		client.MarketClientOpts,
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

	var marketCli clientapi.IMarketClientStruct
	permission.PermissionProxy((clientapi.IMarketClient)(resAPI), &marketCli)

	apiHandles := []rpc.APIHandle{
		{Path: "/rpc/v0", API: &marketCli},
	}
	return rpc.ServeRPC(ctx, cfg, &cfg.API, mux.NewRouter(), 1000, cli2.API_NAMESPACE_MARKET_CLIENT, nil, apiHandles, finishCh)
}