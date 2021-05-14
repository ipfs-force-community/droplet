package main

import (
	"fmt"
	"github.com/filecoin-project/go-fil-markets/discovery"
	discoveryimpl "github.com/filecoin-project/go-fil-markets/discovery/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/client"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus/pkg/market"
	"net"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var log = logging.Logger("main")
func main() {
	app := &cli.App{
		Name:  "venus market",
		Usage: "used for market",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "./messager.toml",
				Usage:   "specify config file",
			},
		},
		Commands: []*cli.Command{
			runCmd,
		},
	}
	app.Version = Version + "--" + GitCommit
	app.Setup()
	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		return
	}

}

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "run venus market",
	Flags: []cli.Flag{
	},
	Action: runAction,
}

func runAction(ctx *cli.Context) error {
	path := ctx.String("config")
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	exit, err := config.ConfigExit(path)
	if err != nil {
		return err
	}

	var cfg *config.Config
	if !exit {
		cfg = config.DefaultConfig()
		err = updateFlag(cfg, ctx)
		if err != nil {
			return err
		}
		err = config.WriteConfig(path, cfg)
		if err != nil {
			return err
		}
	} else {
		cfg, err = config.ReadConfig(path)
		if err != nil {
			return err
		}
		err = updateFlag(cfg, ctx)
		if err != nil {
			return err
		}
	}

	nodeClient, nodeClientCloser, err := client.NewNodeClient(ctx.Context, &cfg.Node)
	if err != nil {
		return err
	}
	defer nodeClientCloser()

	messagerClient, messagerCloser, err := client.NewMessageRPC(ctx.Context, &cfg.MessageService)
	if err != nil {
		return err
	}
	defer messagerCloser()

	lst, err := net.Listen("tcp", cfg.API.Address)
	if err != nil {
		return err
	}

	shutdownChan := make(chan struct{})
	provider := fx.Options(
		//prover
		fx.Supply(cfg, &cfg.DB, &cfg.API, &cfg.JWT, &cfg.Node, &cfg.Log, &cfg.MessageService,),
		fx.Supply(nodeClient),
		fx.Supply(messagerClient),
		fx.Supply((ShutdownChan)(shutdownChan)),
		//db
		fx.Provide(models.SetDataBase),
		fx.Provide(ClientMultiDatastore),
		
		/*
			//ClientMultiDatastore
			ds, err := r.Datastore(ctx, "/client")
			if err != nil {
				return nil, xerrors.Errorf("getting datastore out of repo: %w", err)
			}

			mds, err := multistore.NewMultiDstore(ds)
			if err != nil {
				return nil, err
			}
		 */
		Override(new(dtypes.ClientMultiDstore), modules.ClientMultiDatastore),
		Override(new(dtypes.ClientImportMgr), modules.ClientImportMgr), //		Blockstore: blockstore.Adapt(mds.MultiReadBlockstore()),
		Override(new(dtypes.ClientBlockstore), modules.ClientBlockstore),
		// Shared graphsync (markets, serving chain)
		Override(new(dtypes.Graphsync), modules.Graphsync(config.DefaultFullNode().Client.SimultaneousTransfers)),

		// Markets (common)
		Override(new(*discoveryimpl.Local), modules.NewLocalDiscovery),

		// Markets (retrieval)
		Override(new(discovery.PeerResolver), modules.RetrievalResolver),
		Override(new(retrievalmarket.RetrievalClient), modules.RetrievalClient),
		Override(new(dtypes.ClientDataTransfer), modules.NewClientGraphsyncDataTransfer),

		// Markets (storage)
		Override(new(*market.FundManager), market.NewFundManager),
		Override(new(dtypes.ClientDatastore), modules.NewClientDatastore),
		Override(new(storagemarket.StorageClient), modules.StorageClient),
		Override(new(storagemarket.StorageClientNode), storageadapter.NewClientNodeAdapter),
		Override(HandleMigrateClientFundsKey, modules.HandleMigrateClientFunds),




		fx.Provide(func() net.Listener {
			return lst
		}),
	)

	invoker := fx.Options(
		//invoke
		fx.Invoke(api.RunAPI),
	)
	app := fx.New(provider, invoker)
	if err := app.Start(ctx.Context); err != nil {
		// comment fx.NopLogger few lines above for easier debugging
		return xerrors.Errorf("starting node: %w", err)
	}

	go func() {
		<-shutdownChan
		log.Warn("received shutdown")

		log.Warn("Shutting down...")
		if err := app.Stop(ctx.Context); err != nil {
			log.Errorf("graceful shutting down failed: %s", err)
		}
		log.Warn("Graceful shutdown successful")
	}()

	<-app.Done()
	return nil
}

func updateFlag(cfg *config.Config, ctx *cli.Context) error {
	if ctx.IsSet("auth-url") {
		cfg.JWT.Url = ctx.String("auth-url")
	}

	if ctx.IsSet("node-url") {
		cfg.Node.Url = ctx.String("node-url")
	}

	if ctx.IsSet("node-token") {
		cfg.Node.Token = ctx.String("node-token")
	}

	if ctx.IsSet("db-type") {
		cfg.DB.Type = ctx.String("db-type")
		switch cfg.DB.Type {
		case "sqlite":
			if ctx.IsSet("sqlite-path") {
				cfg.DB.Sqlite.Path = ctx.String("sqlite-path")
			}
		case "mysql":
			if ctx.IsSet("mysql-dsn") {
				cfg.DB.MySql.ConnectionString = ctx.String("mysql-dsn")
			}
		default:
			return xerrors.New("unsupport db type")
		}
	}
	return nil
}

type fxLogger struct {

}

func (l fxLogger) Printf(str string, args ...interface{}) {
	log.Infof(str+"\n", args...)
}
