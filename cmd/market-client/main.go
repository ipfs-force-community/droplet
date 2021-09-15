package main

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-fil-markets/discovery"
	discoveryimpl "github.com/filecoin-project/go-fil-markets/discovery/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
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
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/app/client/apiface"
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

var ExtractApiKey builder.Invoke = 10

func main() {
	app := &cli.App{
		Name:                 "venus-market-client",
		Usage:                "venus-market client",
		Version:              constants.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "repo",
				Value: "~/.marketclient",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "run",
				Usage:  "run market daemon",
				Action: marketClient,
			},
			cli2.ClientCmd,
		},
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
		fmt.Println(cfgPath)
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
		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "venus-market")
		}),
		config.ConfigClientOpts(cfg),
		builder.Override(new(apiface.FullNode), clients2.NodeClient),
		builder.Override(new(clients2.ISinger), clients2.NewWalletClient),
		builder.Override(new(clients2.IMessager), clients2.MessagerClient),
		clients2.ClientsOpts,
		models.DBOptions(false),
		network.NetworkOpts(false, cfg.SimultaneousTransfers),
		paychmgr.PaychOpts,
		fundmgr.FundMgrOpts,

		// global system journal.
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), journal.OpenFilesystemJournal),

		// Markets (common)
		builder.Override(new(*discoveryimpl.Local), client.NewLocalDiscovery),
		builder.Override(new(discovery.PeerResolver), client.RetrievalResolver),
		builder.Override(new(network.ClientDataTransfer), client.NewClientGraphsyncDataTransfer),

		builder.Override(new(client.ClientImportMgr), client.NewClientImportMgr),
		builder.Override(new(storagemarket.BlockstoreAccessor), client.StorageBlockstoreAccessor),

		builder.Override(new(retrievalmarket.BlockstoreAccessor), client.RetrievalBlockstoreAccessor),
		builder.Override(new(retrievalmarket.RetrievalClient), client.RetrievalClient),
		builder.Override(new(storagemarket.StorageClient), client.StorageClient),
		builder.Override(new(storagemarket.StorageClientNode), storageadapter.NewClientNodeAdapter),
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
