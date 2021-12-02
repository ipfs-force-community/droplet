package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"

	metrics2 "github.com/ipfs/go-metrics-interface"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"
	"github.com/ipfs-force-community/venus-common-utils/metrics"

	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/api/impl"
	cli2 "github.com/filecoin-project/venus-market/cli"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/dagstore"
	"github.com/filecoin-project/venus-market/fundmgr"
	minermgr2 "github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/paychmgr"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/filecoin-project/venus-market/retrievalprovider"
	"github.com/filecoin-project/venus-market/rpc"
	"github.com/filecoin-project/venus-market/storageprovider"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"

	_ "github.com/filecoin-project/venus-market/network"

	"github.com/filecoin-project/venus/pkg/constants"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
)

// Invokes are called in the order they are defined.
//nolint:golint
var (
	InitJournalKey = builder.NextInvoke() //nolint
	ExtractApiKey  = builder.NextInvoke()
)

var (
	RepoFlag = &cli.StringFlag{
		Name:    "repo",
		EnvVars: []string{"VENUS_MARKET_PATH"},
		Value:   "~/.venusmarket",
	}

	NodeUrlFlag = &cli.StringFlag{
		Name:  "node-url",
		Usage: "url to connect to daemon service",
	}
	NodeTokenFlag = &cli.StringFlag{
		Name:  "node-token",
		Usage: "node token",
	}

	AuthUrlFlag = &cli.StringFlag{
		Name:  "auth-url",
		Usage: "url to connect to auth service",
	}
	AuthTokeFlag = &cli.StringFlag{
		Name:  "auth-token",
		Usage: "token for connect venus components",
	}

	MessagerUrlFlag = &cli.StringFlag{
		Name:  "messager-url",
		Usage: "url to connect messager service",
	}
	MessagerTokenFlag = &cli.StringFlag{
		Name:   "messager-token",
		Usage:  "messager token",
		Hidden: true,
	}

	SignerTypeFlag = &cli.StringFlag{
		Name:        "signer-type",
		Usage:       "signer service type（wallet, gateway）",
		DefaultText: "wallet",
	}
	SignerUrlFlag = &cli.StringFlag{
		Name:  "signer-url",
		Usage: "used to connect signer service for sign",
	}
	SignerTokenFlag = &cli.StringFlag{
		Name:  "signer-token",
		Usage: "auth token for connect signer service",
	}
	PieceStorageFlag = &cli.StringFlag{
		Name:  "piecestorage",
		Usage: "config storage for piece",
	}
	MysqlDsnFlag = &cli.StringFlag{
		Name:  "mysql-dsn",
		Usage: "mysql connection string",
	}
	MinerListFlag = &cli.StringSliceFlag{
		Name:  "miner",
		Usage: "support miner( f01000:jimmy)",
	}
)

func main() {
	app := &cli.App{
		Name:                 "venus-market",
		Usage:                "venus-market",
		Version:              constants.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			RepoFlag,
		},
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "run market daemon",
				Flags: []cli.Flag{
					NodeUrlFlag,
					NodeTokenFlag,
					AuthUrlFlag,
					AuthTokeFlag,
					MessagerUrlFlag,
					MessagerTokenFlag,
					SignerTypeFlag,
					SignerUrlFlag,
					SignerTokenFlag,
					PieceStorageFlag,
					MysqlDsnFlag,
					MinerListFlag,
				},
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
		builder.Override(new(journal.Journal), func(lc fx.Lifecycle, home config.IHome, disabled journal.DisabledEvents) (journal.Journal, error) {
			return journal.OpenFilesystemJournal(lc, home.MustHomePath(), "venus-market", disabled)
		}),

		builder.Override(new(metrics.MetricsCtx), func() context.Context {
			return metrics2.CtxScope(context.Background(), "venus-market")
		}),
		builder.Override(new(types.ShutdownChan), shutdownChan),
		//config
		config.ConfigServerOpts(cfg),

		// miner manager
		minermgr2.MinerMgrOpts(cfg),

		//clients
		clients.ClientsOpts(true, &cfg.Messager, &cfg.Signer),
		models.DBOptions(true, &cfg.Mysql),
		network.NetworkOpts(true, cfg.SimultaneousTransfers),
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

	return rpc.ServeRPC(ctx, cfg, &cfg.API, api.MarketFullNode(resAPI), finishCh, 1000, "")
}

func flagData(cctx *cli.Context, cfg *config.MarketConfig) error {
	if cctx.IsSet("repo") {
		cfg.HomeDir = cctx.String("repo")
	}

	if cctx.IsSet("node-url") {
		cfg.Node.Url = cctx.String("node-url")
	}

	if cctx.IsSet("messager-url") {
		cfg.Messager.Url = cctx.String("messager-url")
	}

	if cctx.IsSet("auth-url") {
		cfg.AuthNode.Url = cctx.String("auth-url")
	}

	if cctx.IsSet("signer-type") {
		cfg.Signer.SignerType = cctx.String("signer-type")
	}

	if cctx.IsSet("signer-url") {
		cfg.Signer.Url = cctx.String("signer-url")
	}

	if cctx.IsSet("auth-token") {
		cfg.Node.Token = cctx.String("auth-token")
		if len(cfg.AuthNode.Url) > 0 {
			cfg.AuthNode.Token = cctx.String("auth-token")
		}

		if len(cfg.Messager.Url) > 0 {
			cfg.Messager.Token = cctx.String("auth-token")
		}

		if cfg.Signer.SignerType == "gateway" {
			cfg.Signer.Token = cctx.String("auth-token")
		}
	}

	if cctx.IsSet("node-token") {
		cfg.Node.Token = cctx.String("node-token")
	}

	if cctx.IsSet("messager-token") {
		cfg.Messager.Token = cctx.String("messager-token")
	}

	if cctx.IsSet("signer-token") {
		cfg.Signer.Token = cctx.String("signer-token")
	}

	if cctx.IsSet("piecestorage") {
		cfg.PieceStorage = config.PieceStorageString(cctx.String("piecestorage"))
	}

	if cctx.IsSet("mysql-dsn") {
		cfg.Mysql.ConnectionString = cctx.String("mysql-dsn")
	}

	if cctx.IsSet("miner") {
		addrStrs := cctx.StringSlice("miner")
		for _, miners := range addrStrs {
			addrStr := strings.Split(miners, ":")
			addr, err := address.NewFromString(addrStr[0])
			if err != nil {
				return xerrors.Errorf("flag provide a wrong address %s %w", addrStr, err)
			}
			account := ""
			if len(addrStr) > 2 {
				account = addrStr[1]
			}
			cfg.StorageMiners = append(cfg.StorageMiners, config.User{
				Addr:    config.Address(addr),
				Account: account,
			})
		}
	}
	return nil
}
