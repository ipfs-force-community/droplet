package main

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/venus-market/api/impl"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/markets/dealfilter"
	"github.com/filecoin-project/venus-market/markets/retrievaladapter"
	"github.com/filecoin-project/venus-market/markets/storageadapter"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/piece"
	"github.com/filecoin-project/venus-market/sealer"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
	"os"
	"path"
)

// Invokes are called in the order they are defined.
//nolint:golint
const (
	InitJournalKey     = "InitJournalKey"
	HandleDealsKey     = "HandleDealsKey"
	HandleRetrievalKey = "HandleRetrievalKey"
)

func main() {
	app := &cli.App{
		Name:                 "venus-market",
		Usage:                "venus-market",
		Version:              build.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "repo",
				Value: "./venusmarket",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "run",
				Usage:  "run market daemon",
				Action: run,
			},
		},
	}

	app.Setup()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func prepare(cctx *cli.Context) (*config.MarketConfig, error) {
	cfgPath := path.Join(cctx.String("repo"), "config.toml")

	cfg := config.DefaultMarketConfig
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//create
		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, err
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

func run(cctx *cli.Context) error {
	ctx := cctx.Context
	cfg, err := prepare(cctx)
	if err != nil {
		return nil
	}

	shutdownChan := make(chan struct{})
	_, err = utils.New(ctx,
		//config
		utils.Override(new(config.HomeDir), cfg.HomePath),
		utils.Override(new(config.MarketConfig), cfg),
		utils.Override(new(config.Node), &cfg.Node),
		utils.Override(new(config.Messager), &cfg.Messager),
		utils.Override(new(config.Gateway), &cfg.Gateway),
		utils.Override(new(config.PieceStorage), &cfg.PieceStorage),

		//clients
		utils.Override(new(apiface.FullNode), clients.NodeClient),
		utils.Override(new(clients.IMessager), clients.MessagerClient),
		utils.Override(new(clients.IWalletClient), clients.NewWalletClient),
		utils.Override(new(types.ShutdownChan), shutdownChan),

		//database
		models.DBOptions,
		network.NetworkOpts(cfg),
		piece.PieceOpts(cfg),
		sealer.SealerOpts,

		// Markets (retrieval deps)
		utils.Override(new(config.RetrievalPricingFunc), RetrievalPricingFunc(cfg)),

		// Markets (retrieval)
		utils.Override(new(retrievalmarket.RetrievalProviderNode), retrievaladapter.NewRetrievalProviderNode),
		utils.Override(new(rmnet.RetrievalMarketNetwork), RetrievalNetwork),
		utils.Override(new(retrievalmarket.RetrievalProvider), RetrievalProvider), //save to metadata /retrievals/provider
		utils.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(nil)),
		utils.Override(HandleRetrievalKey, HandleRetrieval),

		// Markets (piecestorage)
		utils.Override(new(network.ProviderDataTransfer), NewProviderDAGServiceDataTransfer), //save to metadata /datatransfer/provider/transfers
		utils.Override(new(*storedask.StoredAsk), NewStorageAsk),                             //   save to metadata /deals/provider/piecestorage-ask/latest
		utils.Override(new(config.StorageDealFilter), BasicDealFilter(nil)),
		utils.Override(new(storagemarket.StorageProvider), StorageProvider),
		utils.Override(new(*storageadapter.DealPublisher), storageadapter.NewDealPublisher(cfg)),
		utils.Override(HandleDealsKey, HandleDeals),

		// Config (todo: get a real property system)
		utils.Override(new(config.ConsiderOnlineStorageDealsConfigFunc), NewConsiderOnlineStorageDealsConfigFunc),
		utils.Override(new(config.SetConsiderOnlineStorageDealsConfigFunc), NewSetConsideringOnlineStorageDealsFunc),
		utils.Override(new(config.ConsiderOnlineRetrievalDealsConfigFunc), NewConsiderOnlineRetrievalDealsConfigFunc),
		utils.Override(new(config.SetConsiderOnlineRetrievalDealsConfigFunc), NewSetConsiderOnlineRetrievalDealsConfigFunc),
		utils.Override(new(config.StorageDealPieceCidBlocklistConfigFunc), NewStorageDealPieceCidBlocklistConfigFunc),
		utils.Override(new(config.SetStorageDealPieceCidBlocklistConfigFunc), NewSetStorageDealPieceCidBlocklistConfigFunc),
		utils.Override(new(config.ConsiderOfflineStorageDealsConfigFunc), NewConsiderOfflineStorageDealsConfigFunc),
		utils.Override(new(config.SetConsiderOfflineStorageDealsConfigFunc), NewSetConsideringOfflineStorageDealsFunc),
		utils.Override(new(config.ConsiderOfflineRetrievalDealsConfigFunc), NewConsiderOfflineRetrievalDealsConfigFunc),
		utils.Override(new(config.SetConsiderOfflineRetrievalDealsConfigFunc), NewSetConsiderOfflineRetrievalDealsConfigFunc),
		utils.Override(new(config.ConsiderVerifiedStorageDealsConfigFunc), NewConsiderVerifiedStorageDealsConfigFunc),
		utils.Override(new(config.SetConsiderVerifiedStorageDealsConfigFunc), NewSetConsideringVerifiedStorageDealsFunc),
		utils.Override(new(config.ConsiderUnverifiedStorageDealsConfigFunc), NewConsiderUnverifiedStorageDealsConfigFunc),
		utils.Override(new(config.SetConsiderUnverifiedStorageDealsConfigFunc), NewSetConsideringUnverifiedStorageDealsFunc),
		utils.Override(new(config.SetExpectedSealDurationFunc), NewSetExpectedSealDurationFunc),
		utils.Override(new(config.GetExpectedSealDurationFunc), NewGetExpectedSealDurationFunc),
		utils.Override(new(config.SetMaxDealStartDelayFunc), NewSetMaxDealStartDelayFunc),
		utils.Override(new(config.GetMaxDealStartDelayFunc), NewGetMaxDealStartDelayFunc),

		utils.If(cfg.Filter != "",
			utils.Override(new(config.StorageDealFilter), BasicDealFilter(dealfilter.CliStorageDealFilter(cfg.Filter))),
		),

		utils.If(cfg.RetrievalFilter != "",
			utils.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(dealfilter.CliRetrievalDealFilter(cfg.RetrievalFilter))),
		),
		utils.Override(new(*storageadapter.DealPublisher), storageadapter.NewDealPublisher(cfg)),
		utils.Override(new(storagemarket.StorageProviderNode), storageadapter.NewProviderNodeAdapter(cfg)),
	)
	if err != nil {
		return xerrors.Errorf("initializing node: %w", err)
	}
	finishCh := MonitorShutdown(shutdownChan)
	return serveRPC(ctx, &cfg.API, &impl.MarketNodeImpl{}, finishCh, 1000, "")
}
