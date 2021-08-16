package main

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	sectorstorage "github.com/filecoin-project/lotus/extern/sector-storage"
	"github.com/filecoin-project/lotus/node/modules/lp2p"
	"github.com/filecoin-project/lotus/node/repo"
	"github.com/filecoin-project/lotus/storage/sectorblocks"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/dtypes"
	"github.com/filecoin-project/venus-market/markets/dealfilter"
	"github.com/filecoin-project/venus-market/markets/retrievaladapter"
	"github.com/filecoin-project/venus-market/markets/storageadapter"
	"github.com/filecoin-project/venus-market/network"
	lp2p2 "github.com/filecoin-project/venus-market/network"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
	"os"

	"go.uber.org/fx"
	"time"
)

type special struct{ id int }

//nolint:golint
var (
	DefaultTransportsKey = special{0}  // Libp2p option
	DiscoveryHandlerKey  = special{2}  // Private type
	AddrsFactoryKey      = special{3}  // Libp2p option
	SmuxTransportKey     = special{4}  // Libp2p option
	RelayKey             = special{5}  // Libp2p option
	SecurityKey          = special{6}  // Libp2p option
	BaseRoutingKey       = special{7}  // fx groups + multiret
	NatPortMapKey        = special{8}  // Libp2p option
	ConnectionManagerKey = special{9}  // Libp2p option
	AutoNATSvcKey        = special{10} // Libp2p option
	BandwidthReporterKey = special{11} // Libp2p option
	ConnGaterKey         = special{12} // libp2p option
)

type invoke int

// Invokes are called in the order they are defined.
//nolint:golint
const (
	// InitJournal at position 0 initializes the journal global var as soon as
	// the system starts, so that it's available for all other components.
	InitJournalKey = invoke(iota)
	// miner
	PstoreAddSelfKeysKey
	StartListeningKey
	GetParamsKey
	HandleMigrateProviderFundsKey
	HandleDealsKey
	HandleRetrievalKey
	RunSectorServiceKey

	_nInvokes // keep this last
)

type Settings struct {
	// modules is a map of constructors for DI
	//
	// In most cases the index will be a reflect. Type of element returned by
	// the constructor, but for some 'constructors' it's hard to specify what's
	// the return type should be (or the constructor returns fx group)
	modules map[interface{}]fx.Option

	// invokes are separate from modules as they can't be referenced by return
	// type, and must be applied in correct order
	invokes []fx.Option

	nodeType repo.RepoType

	Base   bool // Base option applied
	Config bool // Config option applied

	enableLibp2pNode bool
}

func main() {
	app := &cli.App{
		Name:                 "venus-market",
		Usage:                "venus-market",
		Version:              build.UserVersion(),
		EnableBashCompletion: true,
		Flags:                []cli.Flag{},
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

func run(cctx *cli.Context) error {
	ctx := cctx.Context
	cfg := config.DefaultMarketConfig

	nodeClient, closer, err := clients.NewNodeClient(ctx, &cfg.Node)
	if err != nil {
		return err
	}
	defer closer()

	shutdownChan := make(chan struct{})
	_, err = New(ctx,
		//config
		Override(new(config.Market), cfg),
		//database
		Override(new(dtypes.StagingDS), MetadataDs),
		Override(new(dtypes.StagingDS), StageingDs),
		Override(new(dtypes.StagingBlockstore), StagingBlockStore),
		Override(new(dtypes.StagingMultiDstore), StagingMultiDatastore),

		//node
		Override(new(api.FullNode), nodeClient),
		Override(new(dtypes.ShutdownChan), shutdownChan),

		//libp2p
		// Host dependencies
		Override(new(crypto.PrivKey), lp2p2.PrivKey),
		Override(new(crypto.PubKey), crypto.PrivKey.GetPublic),
		Override(new(peer.ID), peer.IDFromPublicKey),
		Override(new(peerstore.Peerstore), pstoremem.NewPeerstore),
		Override(PstoreAddSelfKeysKey, network.PstoreAddSelfKeys),
		Override(StartListeningKey, network.StartListening(cfg.Libp2p.ListenAddresses)),
		Override(AddrsFactoryKey, network.AddrsFactory(cfg.Libp2p.AnnounceAddresses, cfg.Libp2p.NoAnnounceAddresses)),
		Override(DefaultTransportsKey, network.DefaultTransports),
		Override(SmuxTransportKey, network.SmuxTransport(true)),
		Override(RelayKey, network.NoRelay()),
		Override(SecurityKey, network.Security(true, false)),

		// Host
		Override(new(host.Host), network.Host),
		// Markets
		Override(new(dtypes.StagingDAG), StagingDAG),
		Override(new(dtypes.StagingGraphsync), StagingGraphsync(cfg.SimultaneousTransfers)),
		Override(new(dtypes.ProviderPieceStore), NewProviderPieceStore),
		Override(new(*sectorblocks.SectorBlocks), sectorblocks.NewSectorBlocks),

		// Markets (retrieval deps)
		Override(new(sectorstorage.PieceProvider), sectorstorage.NewPieceProvider),
		Override(new(dtypes.RetrievalPricingFunc), RetrievalPricingFunc(cfg)),

		// Markets (retrieval)
		Override(new(retrievalmarket.RetrievalProviderNode), retrievaladapter.NewRetrievalProviderNode),
		Override(new(rmnet.RetrievalMarketNetwork), RetrievalNetwork),
		Override(new(retrievalmarket.RetrievalProvider), RetrievalProvider),
		Override(new(dtypes.RetrievalDealFilter), RetrievalDealFilter(nil)),
		Override(HandleRetrievalKey, HandleRetrieval),

		// Markets (storage)
		Override(new(dtypes.ProviderDataTransfer), NewProviderDAGServiceDataTransfer),
		Override(new(*storedask.StoredAsk), NewStorageAsk),
		Override(new(dtypes.StorageDealFilter), BasicDealFilter(nil)),
		Override(new(storagemarket.StorageProvider), StorageProvider),
		Override(new(*storageadapter.DealPublisher), storageadapter.NewDealPublisher(nil, storageadapter.PublishMsgConfig{})),
		Override(HandleMigrateProviderFundsKey, HandleMigrateProviderFunds),
		Override(HandleDealsKey, HandleDeals),

		// Config (todo: get a real property system)
		Override(new(dtypes.ConsiderOnlineStorageDealsConfigFunc), NewConsiderOnlineStorageDealsConfigFunc),
		Override(new(dtypes.SetConsiderOnlineStorageDealsConfigFunc), NewSetConsideringOnlineStorageDealsFunc),
		Override(new(dtypes.ConsiderOnlineRetrievalDealsConfigFunc), NewConsiderOnlineRetrievalDealsConfigFunc),
		Override(new(dtypes.SetConsiderOnlineRetrievalDealsConfigFunc), NewSetConsiderOnlineRetrievalDealsConfigFunc),
		Override(new(dtypes.StorageDealPieceCidBlocklistConfigFunc), NewStorageDealPieceCidBlocklistConfigFunc),
		Override(new(dtypes.SetStorageDealPieceCidBlocklistConfigFunc), NewSetStorageDealPieceCidBlocklistConfigFunc),
		Override(new(dtypes.ConsiderOfflineStorageDealsConfigFunc), NewConsiderOfflineStorageDealsConfigFunc),
		Override(new(dtypes.SetConsiderOfflineStorageDealsConfigFunc), NewSetConsideringOfflineStorageDealsFunc),
		Override(new(dtypes.ConsiderOfflineRetrievalDealsConfigFunc), NewConsiderOfflineRetrievalDealsConfigFunc),
		Override(new(dtypes.SetConsiderOfflineRetrievalDealsConfigFunc), NewSetConsiderOfflineRetrievalDealsConfigFunc),
		Override(new(dtypes.ConsiderVerifiedStorageDealsConfigFunc), NewConsiderVerifiedStorageDealsConfigFunc),
		Override(new(dtypes.SetConsiderVerifiedStorageDealsConfigFunc), NewSetConsideringVerifiedStorageDealsFunc),
		Override(new(dtypes.ConsiderUnverifiedStorageDealsConfigFunc), NewConsiderUnverifiedStorageDealsConfigFunc),
		Override(new(dtypes.SetConsiderUnverifiedStorageDealsConfigFunc), NewSetConsideringUnverifiedStorageDealsFunc),
		Override(new(dtypes.SetExpectedSealDurationFunc), NewSetExpectedSealDurationFunc),
		Override(new(dtypes.GetExpectedSealDurationFunc), NewGetExpectedSealDurationFunc),
		Override(new(dtypes.SetMaxDealStartDelayFunc), NewSetMaxDealStartDelayFunc),
		Override(new(dtypes.GetMaxDealStartDelayFunc), NewGetMaxDealStartDelayFunc),

		If(cfg.Filter != "",
			Override(new(dtypes.StorageDealFilter), BasicDealFilter(dealfilter.CliStorageDealFilter(cfg.Filter))),
		),

		If(cfg.RetrievalFilter != "",
			Override(new(dtypes.RetrievalDealFilter), RetrievalDealFilter(dealfilter.CliRetrievalDealFilter(cfg.RetrievalFilter))),
		),
		Override(new(*storageadapter.DealPublisher), storageadapter.NewDealPublisher(cfg, storageadapter.PublishMsgConfig{
			Period:         time.Duration(cfg.PublishMsgPeriod),
			MaxDealsPerMsg: cfg.MaxDealsPerPublishMsg,
		})),
		Override(new(storagemarket.StorageProviderNode), storageadapter.NewProviderNodeAdapter(cfg)),
	)
	if err != nil {
		return xerrors.Errorf("initializing node: %w", err)
	}
	finishCh := MonitorShutdown(shutdownChan)
	<-finishCh // fires when shutdown is complete.
	return nil
}
