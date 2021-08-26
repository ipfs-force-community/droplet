package main

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	piecefilestore "github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/constants"
	"github.com/filecoin-project/venus-market/journal"
	"github.com/filecoin-project/venus-market/markets"
	marketevents "github.com/filecoin-project/venus-market/markets/loggers"
	"github.com/filecoin-project/venus-market/markets/pricing"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/network"
	types2 "github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"
	"os"
	"path/filepath"
	"time"
)

var (
	log = logging.Logger("modules")
)

// RetrievalPricingFunc configures the pricing function to use for retrieval deals.
func RetrievalPricingFunc(cfg *config.MarketConfig) func(_ config.ConsiderOnlineRetrievalDealsConfigFunc,
	_ config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalPricingFunc {

	return func(_ config.ConsiderOnlineRetrievalDealsConfigFunc,
		_ config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalPricingFunc {
		if cfg.RetrievalPricing.Strategy == config.RetrievalPricingExternalMode {
			return pricing.ExternalRetrievalPricingFunc(cfg.RetrievalPricing.External.Path)
		}

		return retrievalimpl.DefaultPricingFunc(cfg.RetrievalPricing.Default.VerifiedDealsFreeTransfer)
	}
}

// NewProviderDAGServiceDataTransfer returns a data transfer manager that just
// uses the provider's Staging DAG service for transfers
func NewProviderDAGServiceDataTransfer(lc fx.Lifecycle, h host.Host, homeDir config.HomeDir, gs network.StagingGraphsync, ds models.MetadataDS, cfg *config.MarketConfig) (network.ProviderDataTransfer, error) {
	net := dtnet.NewFromLibp2pHost(h)

	dtDs := namespace.Wrap(ds, datastore.NewKey("/datatransfer/provider/transfers"))
	transport := dtgstransport.NewTransport(h.ID(), gs)
	err := os.MkdirAll(filepath.Join(string(homeDir), "data-transfer"), 0755) //nolint: gosec
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	dt, err := dtimpl.NewDataTransfer(dtDs, filepath.Join(string(homeDir), "data-transfer"), net, transport)
	if err != nil {
		return nil, err
	}

	dt.OnReady(marketevents.ReadyLogger("provider data transfer"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			dt.SubscribeToEvents(marketevents.DataTransferLogger)
			return dt.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return dt.Stop(ctx)
		},
	})
	return dt, nil
}

func NewStorageAsk(ctx metrics.MetricsCtx,
	fapi apiface.FullNode,
	ds models.MetadataDS,
	minerAddress types2.MinerAddress,
	spn storagemarket.StorageProviderNode) (*storedask.StoredAsk, error) {

	mi, err := fapi.StateMinerInfo(ctx, address.Address(minerAddress), types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	providerDs := namespace.Wrap(ds, datastore.NewKey("/deals/provider"))
	return storedask.NewStoredAsk(namespace.Wrap(providerDs, datastore.NewKey("/storage-ask")), datastore.NewKey("latest"), spn, address.Address(minerAddress),
		storagemarket.MaxPieceSize(abi.PaddedPieceSize(mi.SectorSize)))
}

func BasicDealFilter(user config.StorageDealFilter) func(onlineOk config.ConsiderOnlineStorageDealsConfigFunc,
	offlineOk config.ConsiderOfflineStorageDealsConfigFunc,
	verifiedOk config.ConsiderVerifiedStorageDealsConfigFunc,
	unverifiedOk config.ConsiderUnverifiedStorageDealsConfigFunc,
	blocklistFunc config.StorageDealPieceCidBlocklistConfigFunc,
	expectedSealTimeFunc config.GetExpectedSealDurationFunc,
	startDelay config.GetMaxDealStartDelayFunc,
	spn storagemarket.StorageProviderNode) config.StorageDealFilter {
	return func(onlineOk config.ConsiderOnlineStorageDealsConfigFunc,
		offlineOk config.ConsiderOfflineStorageDealsConfigFunc,
		verifiedOk config.ConsiderVerifiedStorageDealsConfigFunc,
		unverifiedOk config.ConsiderUnverifiedStorageDealsConfigFunc,
		blocklistFunc config.StorageDealPieceCidBlocklistConfigFunc,
		expectedSealTimeFunc config.GetExpectedSealDurationFunc,
		startDelay config.GetMaxDealStartDelayFunc,
		spn storagemarket.StorageProviderNode) config.StorageDealFilter {

		return func(ctx context.Context, deal storagemarket.MinerDeal) (bool, string, error) {
			b, err := onlineOk()
			if err != nil {
				return false, "miner error", err
			}

			if deal.Ref != nil && deal.Ref.TransferType != storagemarket.TTManual && !b {
				log.Warnf("online piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not considering online piecestorage deals", nil
			}

			b, err = offlineOk()
			if err != nil {
				return false, "miner error", err
			}

			if deal.Ref != nil && deal.Ref.TransferType == storagemarket.TTManual && !b {
				log.Warnf("offline piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not accepting offline piecestorage deals", nil
			}

			b, err = verifiedOk()
			if err != nil {
				return false, "miner error", err
			}

			if deal.Proposal.VerifiedDeal && !b {
				log.Warnf("verified piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not accepting verified piecestorage deals", nil
			}

			b, err = unverifiedOk()
			if err != nil {
				return false, "miner error", err
			}

			if !deal.Proposal.VerifiedDeal && !b {
				log.Warnf("unverified piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not accepting unverified piecestorage deals", nil
			}

			blocklist, err := blocklistFunc()
			if err != nil {
				return false, "miner error", err
			}

			for idx := range blocklist {
				if deal.Proposal.PieceCID.Equals(blocklist[idx]) {
					log.Warnf("piece CID in proposal %s is blocklisted; rejecting piecestorage deal proposal from client: %s", deal.Proposal.PieceCID, deal.Client.String())
					return false, fmt.Sprintf("miner has blocklisted piece CID %s", deal.Proposal.PieceCID), nil
				}
			}

			sealDuration, err := expectedSealTimeFunc()
			if err != nil {
				return false, "miner error", err
			}

			sealEpochs := sealDuration / (time.Duration(constants.BlockDelaySecs) * time.Second)
			_, ht, err := spn.GetChainHead(ctx)
			if err != nil {
				return false, "failed to get chain head", err
			}
			earliest := abi.ChainEpoch(sealEpochs) + ht
			if deal.Proposal.StartEpoch < earliest {
				log.Warnw("proposed deal would start before sealing can be completed; rejecting piecestorage deal proposal from client", "piece_cid", deal.Proposal.PieceCID, "client", deal.Client.String(), "seal_duration", sealDuration, "earliest", earliest, "curepoch", ht)
				return false, fmt.Sprintf("cannot seal a sector before %s", deal.Proposal.StartEpoch), nil
			}

			sd, err := startDelay()
			if err != nil {
				return false, "miner error", err
			}

			// Reject if it's more than 7 days in the future
			// TODO: read from cfg
			maxStartEpoch := earliest + abi.ChainEpoch(uint64(sd.Seconds())/constants.BlockDelaySecs)
			if deal.Proposal.StartEpoch > maxStartEpoch {
				return false, fmt.Sprintf("deal start epoch is too far in the future: %s > %s", deal.Proposal.StartEpoch, maxStartEpoch), nil
			}

			if user != nil {
				return user(ctx, deal)
			}

			return true, "", nil
		}
	}
}

func RetrievalNetwork(h host.Host) rmnet.RetrievalMarketNetwork {
	return rmnet.NewFromLibp2pHost(h)
}

func StorageProvider(
	homeDir config.HomeDir,
	minerAddress types2.MinerAddress,
	storedAsk *storedask.StoredAsk,
	h host.Host,
	ds models.MetadataDS,
	mds models.StagingMultiDstore,
	pieceStore piecestore.PieceStore,
	dataTransfer network.ProviderDataTransfer,
	spn storagemarket.StorageProviderNode,
	df config.StorageDealFilter,
) (storagemarket.StorageProvider, error) {
	net := smnet.NewFromLibp2pHost(h)
	store, err := piecefilestore.NewLocalFileStore(piecefilestore.OsPath(string(homeDir)))
	if err != nil {
		return nil, err
	}

	opt := storageimpl.CustomDealDecisionLogic(storageimpl.DealDeciderFunc(df))

	return storageimpl.NewProvider(net, namespace.Wrap(ds, datastore.NewKey("/deals/provider")), store, mds, pieceStore, dataTransfer, spn, address.Address(minerAddress), storedAsk, opt)
}

func HandleDeals(mctx metrics.MetricsCtx, lc fx.Lifecycle, host host.Host, h storagemarket.StorageProvider, j journal.Journal) {
	ctx := metrics.LifecycleCtx(mctx, lc)
	h.OnReady(marketevents.ReadyLogger("piecestorage provider"))
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			h.SubscribeToEvents(marketevents.StorageProviderLogger)

			evtType := j.RegisterEventType("markets/piecestorage/provider", "state_change")
			h.SubscribeToEvents(markets.StorageProviderJournaler(j, evtType))

			return h.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return h.Stop()
		},
	})
}

// RetrievalProvider creates a new retrieval provider attached to the provider blockstore
func RetrievalProvider(
	maddr types2.MinerAddress,
	adapter retrievalmarket.RetrievalProviderNode,
	netwk rmnet.RetrievalMarketNetwork,
	ds models.MetadataDS,
	pieceStore piecestore.PieceStore,
	mds models.StagingMultiDstore,
	dt network.ProviderDataTransfer,
	pricingFnc config.RetrievalPricingFunc,
	userFilter config.RetrievalDealFilter,
) (retrievalmarket.RetrievalProvider, error) {
	opt := retrievalimpl.DealDeciderOpt(retrievalimpl.DealDecider(userFilter))
	return retrievalimpl.NewProvider(address.Address(maddr), adapter, netwk, pieceStore, mds, dt, namespace.Wrap(ds, datastore.NewKey("/retrievals/provider")),
		retrievalimpl.RetrievalPricingFunc(pricingFnc), opt)
}

func RetrievalDealFilter(userFilter config.RetrievalDealFilter) func(onlineOk config.ConsiderOnlineRetrievalDealsConfigFunc,
	offlineOk config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalDealFilter {
	return func(onlineOk config.ConsiderOnlineRetrievalDealsConfigFunc,
		offlineOk config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalDealFilter {
		return func(ctx context.Context, state retrievalmarket.ProviderDealState) (bool, string, error) {
			b, err := onlineOk()
			if err != nil {
				return false, "miner error", err
			}

			if !b {
				log.Warn("online retrieval deal consideration disabled; rejecting retrieval deal proposal from client")
				return false, "miner is not accepting online retrieval deals", nil
			}

			b, err = offlineOk()
			if err != nil {
				return false, "miner error", err
			}

			if !b {
				log.Info("offline retrieval has not been implemented yet")
			}

			if userFilter != nil {
				return userFilter(ctx, state)
			}

			return true, "", nil
		}
	}
}

func HandleRetrieval(host host.Host,
	lc fx.Lifecycle,
	m retrievalmarket.RetrievalProvider,
	j journal.Journal,
) {
	m.OnReady(marketevents.ReadyLogger("retrieval provider"))
	lc.Append(fx.Hook{

		OnStart: func(ctx context.Context) error {
			m.SubscribeToEvents(marketevents.RetrievalProviderLogger)

			evtType := j.RegisterEventType("markets/retrieval/provider", "state_change")
			m.SubscribeToEvents(markets.RetrievalProviderJournaler(j, evtType))

			return m.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return m.Stop()
		},
	})
}

func NewConsiderOnlineStorageDealsConfigFunc(cfg *config.MarketConfig) (config.ConsiderOnlineStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOnlineStorageDeals, nil
	}, nil
}

func NewSetConsideringOnlineStorageDealsFunc(cfg *config.MarketConfig) (config.SetConsiderOnlineStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOnlineStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderOnlineRetrievalDealsConfigFunc(cfg *config.MarketConfig) (config.ConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOnlineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOnlineRetrievalDealsConfigFunc(cfg *config.MarketConfig) (config.SetConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOnlineRetrievalDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewStorageDealPieceCidBlocklistConfigFunc(cfg *config.MarketConfig) (config.StorageDealPieceCidBlocklistConfigFunc, error) {
	return func() (out []cid.Cid, err error) {
		return cfg.PieceCidBlocklist, nil
	}, nil
}

func NewSetStorageDealPieceCidBlocklistConfigFunc(cfg *config.MarketConfig) (config.SetStorageDealPieceCidBlocklistConfigFunc, error) {
	return func(blocklist []cid.Cid) (err error) {
		cfg.PieceCidBlocklist = blocklist
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineStorageDealsConfigFunc(cfg *config.MarketConfig) (config.ConsiderOfflineStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOfflineStorageDeals, nil
	}, nil
}

func NewSetConsideringOfflineStorageDealsFunc(cfg *config.MarketConfig) (config.SetConsiderOfflineStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOfflineStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineRetrievalDealsConfigFunc(cfg *config.MarketConfig) (config.ConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOfflineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOfflineRetrievalDealsConfigFunc(cfg *config.MarketConfig) (config.SetConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOfflineRetrievalDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderVerifiedStorageDealsConfigFunc(cfg *config.MarketConfig) (config.ConsiderVerifiedStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderVerifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringVerifiedStorageDealsFunc(cfg *config.MarketConfig) (config.SetConsiderVerifiedStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderVerifiedStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderUnverifiedStorageDealsConfigFunc(cfg *config.MarketConfig) (config.ConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderUnverifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringUnverifiedStorageDealsFunc(cfg *config.MarketConfig) (config.SetConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderUnverifiedStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewSetExpectedSealDurationFunc(cfg *config.MarketConfig) (config.SetExpectedSealDurationFunc, error) {
	return func(delay time.Duration) (err error) {
		cfg.ExpectedSealDuration = config.Duration(delay)
		return config.SaveConfig(cfg)
	}, nil
}

func NewGetExpectedSealDurationFunc(cfg *config.MarketConfig) (config.GetExpectedSealDurationFunc, error) {
	return func() (out time.Duration, err error) {
		return time.Duration(cfg.ExpectedSealDuration), nil
	}, nil
}

func NewSetMaxDealStartDelayFunc(cfg *config.MarketConfig) (config.SetMaxDealStartDelayFunc, error) {
	return func(delay time.Duration) (err error) {
		cfg.MaxDealStartDelay = config.Duration(delay)
		return config.SaveConfig(cfg)
	}, nil
}

func NewGetMaxDealStartDelayFunc(cfg *config.MarketConfig) (config.GetMaxDealStartDelayFunc, error) {
	return func() (out time.Duration, err error) {
		return time.Duration(cfg.MaxDealStartDelay), nil
	}, nil
}
