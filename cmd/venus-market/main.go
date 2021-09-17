package main

import (
	"context"
	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/api/impl"
	"github.com/filecoin-project/venus-market/builder"
	cli2 "github.com/filecoin-project/venus-market/cli"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/fundmgr"
	"github.com/filecoin-project/venus-market/journal"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/paychmgr"
	"github.com/filecoin-project/venus-market/piece"
	"github.com/filecoin-project/venus-market/retrievaladapter"
	"github.com/filecoin-project/venus-market/rpc"
	"github.com/filecoin-project/venus-market/sealer"
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

// Invokes are called in the order they are defined.
//nolint:golint
var (
	InitJournalKey builder.Invoke = builder.NextInvoke() //nolint
	ExtractApiKey  builder.Invoke = builder.NextInvoke()
)

func main() {
	app := &cli.App{
		Name:                 "venus-market",
		Usage:                "venus-market",
		Version:              constants.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "repo",
				Value: "~/.venusmarket",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "run",
				Usage:  "run market daemon",
				Action: daemon,
			},
			cli2.PiecesCmd,
			cli2.RetrievalDealsCmd,
			cli2.StorageDealsCmd,
			cli2.ActorCmd,
			cli2.NetCmd,
			cli2.DataTransfersCmd,
			cli2.DagstoreCmd,
		},
	}

	app.Setup()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func prepare(cctx *cli.Context) (*config.MarketConfig, error) {
	cfg := config.DefaultMarketConfig
	cfg.HomeDir = cctx.String("repo")
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//create
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

func daemon(cctx *cli.Context) error {
	utils.SetupLogLevels()
	ctx := cctx.Context
	cfg, err := prepare(cctx)
	if err != nil {
		return err
	}

	resAPI := &impl.MarketNodeImpl{}
	shutdownChan := make(chan struct{})
	_, err = builder.New(ctx,
		//defaults
		builder.Override(new(journal.DisabledEvents), journal.EnvDisabledEvents),
		builder.Override(new(journal.Journal), journal.OpenFilesystemJournal),

		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "venus-market")
		}),
		builder.Override(new(types.ShutdownChan), shutdownChan),
		//config
		config.ConfigServerOpts(cfg),
		//clients
		clients.ClientsOpts(true),

		models.DBOptions(true),
		network.NetworkOpts(true, cfg.SimultaneousTransfers),
		piece.PieceOpts(cfg),
		fundmgr.FundMgrOpts,
		sealer.SealerOpts,
		paychmgr.PaychOpts,
		// Markets
		storageadapter.StorageProviderOpts(cfg),
		retrievaladapter.RetrievalProviderOpts(cfg),

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

	return rpc.ServeRPC(ctx, cfg, &cfg.API, api.MarketFullNode(resAPI), finishCh, 1000, "")
}
