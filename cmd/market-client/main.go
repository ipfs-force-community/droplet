package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"os"

	metrics2 "github.com/ipfs/go-metrics-interface"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-market/api"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/api/impl"
	cli2 "github.com/filecoin-project/venus-market/cli"
	"github.com/filecoin-project/venus-market/client"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/fundmgr"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/paychmgr"
	"github.com/filecoin-project/venus-market/rpc"
	"github.com/filecoin-project/venus-market/storageprovider"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"

	"github.com/filecoin-project/venus/pkg/constants"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
)

var ExtractApiKey = builder.NextInvoke()

var (
	RepoFlag = &cli.StringFlag{
		Name:    "repo",
		EnvVars: []string{"VENUS_MARKET_CLIENT_PATH"},
		Value:   "~/.marketclient",
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
		Usage: "url to connect the venus-messager service of the chain service layer",
	}

	AuthTokenFlag = &cli.StringFlag{
		Name:  "auth-token",
		Usage: "token used to connect venus chain service components, eg. venus-meassger, venus",
	}

	SignerUrlFlag = &cli.StringFlag{
		Name:  "signer-url",
		Usage: "used to connect signer service for sign",
	}
	SignerTokenFlag = &cli.StringFlag{
		Name:  "signer-token",
		Usage: "auth token for connect signer service",
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
	}

	app := &cli.App{
		Name:                 "market-client",
		Usage:                "venus stores or retrieves the market client",
		Version:              constants.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			RepoFlag,
		},
		Commands: append(
			localCommand,
			&cli.Command{
				Name: "run",
				Usage: "run market client daemon,(1) connect full node service: ./market-client run --node-url=<...> --node-token=<...> --addr=<WALLET_ADDR>;" +
					"(2) connect venus shared service: ./market-client run --node-url=<...> --messager-url=<...> --auth-token=<...>  --signer-url=<...> --signer-token=<...> --addr=<WALLET_ADDR>.",
				Flags: []cli.Flag{
					NodeUrlFlag,
					NodeTokenFlag,
					MessagerUrlFlag,
					AuthTokenFlag,
					SignerUrlFlag,
					SignerTokenFlag,
					DefaultAddressFlag,
				},
				Action: marketClient,
			}),
	}

	app.Setup()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func flagData(cctx *cli.Context, cfg *config.MarketClientConfig) error {
	if cctx.IsSet("repo") {
		cfg.HomeDir = cctx.String("repo")
	}

	if cctx.IsSet("node-url") {
		cfg.Node.Url = cctx.String("node-url")
	}

	if cctx.IsSet("node-token") {
		cfg.Node.Token = cctx.String("node-token")
	}

	if cctx.IsSet("messager-url") {
		if !cctx.IsSet("auth-token") {
			return xerrors.Errorf("the auth-token must be set when connecting to the venus chain service")
		}

		cfg.Node.Token = cctx.String("auth-token")

		cfg.Messager.Url = cctx.String("messager-url")
		cfg.Messager.Token = cctx.String("auth-token")
	}

	if cctx.IsSet("signer-url") {
		if !cctx.IsSet("signer-token") {
			return xerrors.Errorf("signer-url is set, but signer-token is not set")
		}

		cfg.Signer.SignerType = "wallet"
		cfg.Signer.Url = cctx.String("signer-url")
		cfg.Signer.Token = cctx.String("signer-token")
	}

	if cctx.IsSet("addr") {
		addr, err := address.NewFromString(cctx.String("addr"))
		if err != nil {
			return err
		}
		fmt.Println("set default address ", addr.String())
		cfg.DefaultMarketAddress = config.Address(addr)
	}

	return nil
}

func prepare(cctx *cli.Context) (*config.MarketClientConfig, error) {
	cfg := config.DefaultMarketClientConfig
	cfg.HomeDir = cctx.String("repo")
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// create
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, xerrors.Errorf("parser data from flag %w", err)
		}

		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, xerrors.Errorf("save config to %s %w", cfgPath, err)
		}
	} else if err == nil {
		// loadConfig
		err = config.LoadConfig(cfgPath, cfg)
		if err != nil {
			return nil, err
		}
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, xerrors.Errorf("parser data from flag %w", err)
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
	resAPI := &impl.MarketClientNodeImpl{}
	shutdownChan := make(chan struct{})
	_, err = builder.New(ctx,
		// defaults
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), func(lc fx.Lifecycle, home config.IHome, disabled journal.DisabledEvents) (journal.Journal, error) {
			return journal.OpenFilesystemJournal(lc, home.MustHomePath(), "market-client", disabled)
		}),

		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "venus-market")
		}),
		builder.Override(new(types.ShutdownChan), shutdownChan),

		config.ConfigClientOpts(cfg),

		clients2.ClientsOpts(false, "", &cfg.Messager, &cfg.Signer),
		models.DBOptions(false, nil),
		network.NetworkOpts(false, cfg.SimultaneousTransfers),
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
		return xerrors.Errorf("initializing node: %w", err)
	}
	finishCh := utils.MonitorShutdown(shutdownChan)
	return rpc.ServeRPC(ctx, cfg, &cfg.API, mux.NewRouter(), 1000, cli2.API_NAMESPACE_MARKET_CLIENT, "", (api.MarketClientNode)(resAPI), finishCh)
}
