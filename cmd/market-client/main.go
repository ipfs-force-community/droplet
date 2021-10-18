package main

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/api"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/api/impl"
	"github.com/filecoin-project/venus-market/builder"
	cli2 "github.com/filecoin-project/venus-market/cli"
	"github.com/filecoin-project/venus-market/client"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/fundmgr"
	"github.com/filecoin-project/venus-market/journal"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/paychmgr"
	"github.com/filecoin-project/venus-market/rpc"
	"github.com/filecoin-project/venus-market/storageadapter"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/pkg/constants"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	metrics2 "github.com/ipfs/go-metrics-interface"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"log"
	"os"
)

var ExtractApiKey builder.Invoke = builder.NextInvoke()

var (
	RepoFlag = &cli.StringFlag{
		Name:  "repo",
		EnvVars: []string{"VENUS_MARKET_CLIENT_PATH"},
		Value: "~/.marketclient",
	}
	NodeUrlFlag = &cli.StringFlag{
		Name:  "node-url",
		Usage: "url to connect to daemon service",
	}

	MessagerUrlFlag = &cli.StringFlag{
		Name:  "messager-url",
		Usage: "url to connect messager service",
	}

	AuthTokenFlag = &cli.StringFlag{
		Name:  "auth-token",
		Usage: "token for connect venus componets, this flag can set token for messager and node",
	}

	MessagerTokenFlag = &cli.StringFlag{
		Name:  "messager-token",
		Usage: "token for connect venus messager， if specify this flag ,override token set by venus-auth flag ",
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
	app := &cli.App{
		Name:                 "venus-market-client",
		Usage:                "venus-market client",
		Version:              constants.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			RepoFlag,
		},
		Commands: append(cli2.ClientCmds, &cli.Command{
			Name:  "run",
			Usage: "run market daemon",
			Flags: []cli.Flag{
				NodeUrlFlag,
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

func prepare(cctx *cli.Context) (*config.MarketClientConfig, error) {
	cfg := config.DefaultMarketClientConfig
	cfg.HomeDir = cctx.String("repo")
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//create
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, xerrors.Errorf("parser data from flag %w", err)
		}

		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, xerrors.Errorf("save config to %s %w", cfgPath, err)
		}
	} else if err == nil {
		//loadConfig
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
		//defaults
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), journal.OpenFilesystemJournal),

		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "venus-market")
		}),
		builder.Override(new(types.ShutdownChan), shutdownChan),

		config.ConfigClientOpts(cfg),

		clients2.ClientsOpts(false, &cfg.Messager, &cfg.Signer),
		models.DBOptions(false),
		network.NetworkOpts(false, cfg.SimultaneousTransfers),
		paychmgr.PaychOpts,
		fundmgr.FundMgrOpts,
		storageadapter.StorageClientOpts,
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
	return rpc.ServeRPC(ctx, cfg, &cfg.API, (api.MarketClientNode)(resAPI), finishCh, 1000, "")
}

func flagData(cctx *cli.Context, cfg *config.MarketClientConfig) error {
	if cctx.IsSet("repo") {
		cfg.HomeDir = cctx.String("repo")
	}
	if cctx.IsSet("node-url") {
		cfg.Node.Url = cctx.String("node-url")
	}
	if cctx.IsSet("auth-token") {
		cfg.Node.Token = cctx.String("auth-token")
	}

	if cctx.IsSet("messager-url") {
		cfg.Messager.Url = cctx.String("messager-url")
	}
	if cctx.IsSet("auth-token") {
		cfg.Messager.Token = cctx.String("auth-token")
	}
	if cctx.IsSet("messager-token") {
		cfg.Messager.Token = cctx.String("messager-token")
	}

	if cctx.IsSet("signer-url") {
		cfg.Signer.Url = cctx.String("signer-url")
	}
	if cctx.IsSet("signer-token") {
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
