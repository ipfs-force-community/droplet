package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	piecefilestore "github.com/filecoin-project/go-fil-markets/filestore"
	piecestoreimpl "github.com/filecoin-project/go-fil-markets/piecestore/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/v1api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/journal"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/constants"
	"github.com/filecoin-project/venus-market/dtypes"
	"github.com/filecoin-project/venus-market/markets"
	marketevents "github.com/filecoin-project/venus-market/markets/loggers"
	"github.com/filecoin-project/venus-market/markets/pricing"
	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	graphsync "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"os"
	"path/filepath"
	"time"
)

var (
	log = logging.Logger("modules")
)

// RetrievalPricingFunc configures the pricing function to use for retrieval deals.
func RetrievalPricingFunc(cfg *config.Market) func(_ dtypes.ConsiderOnlineRetrievalDealsConfigFunc,
	_ dtypes.ConsiderOfflineRetrievalDealsConfigFunc) dtypes.RetrievalPricingFunc {

	return func(_ dtypes.ConsiderOnlineRetrievalDealsConfigFunc,
		_ dtypes.ConsiderOfflineRetrievalDealsConfigFunc) dtypes.RetrievalPricingFunc {
		if cfg.RetrievalPricing.Strategy == config.RetrievalPricingExternalMode {
			return pricing.ExternalRetrievalPricingFunc(cfg.RetrievalPricing.External.Path)
		}

		return retrievalimpl.DefaultPricingFunc(cfg.RetrievalPricing.Default.VerifiedDealsFreeTransfer)
	}
}

// NewProviderDAGServiceDataTransfer returns a data transfer manager that just
// uses the provider's Staging DAG service for transfers
func NewProviderDAGServiceDataTransfer(lc fx.Lifecycle, h host.Host, gs dtypes.StagingGraphsync, ds dtypes.MetadataDS, cfg *config.Market) (dtypes.ProviderDataTransfer, error) {
	net := dtnet.NewFromLibp2pHost(h)

	dtDs := namespace.Wrap(ds, datastore.NewKey("/datatransfer/provider/transfers"))
	transport := dtgstransport.NewTransport(h.ID(), gs)
	err := os.MkdirAll(filepath.Join(cfg.HomeDir, "data-transfer"), 0755) //nolint: gosec
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	dt, err := dtimpl.NewDataTransfer(dtDs, filepath.Join(cfg.HomeDir, "data-transfer"), net, transport)
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

func NewStorageAsk(ctx helpers.MetricsCtx, fapi v1api.FullNode, ds dtypes.MetadataDS, minerAddress dtypes.MinerAddress, spn storagemarket.StorageProviderNode) (*storedask.StoredAsk, error) {

	mi, err := fapi.StateMinerInfo(ctx, address.Address(minerAddress), types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	providerDs := namespace.Wrap(ds, datastore.NewKey("/deals/provider"))
	// legacy this was mistake where this key was place -- so we move the legacy key if need be
	err = shared.MoveKey(providerDs, "/latest-ask", "/storage-ask/latest")
	if err != nil {
		return nil, err
	}
	return storedask.NewStoredAsk(namespace.Wrap(providerDs, datastore.NewKey("/storage-ask")), datastore.NewKey("latest"), spn, address.Address(minerAddress),
		storagemarket.MaxPieceSize(abi.PaddedPieceSize(mi.SectorSize)))
}

func BasicDealFilter(user dtypes.StorageDealFilter) func(onlineOk dtypes.ConsiderOnlineStorageDealsConfigFunc,
	offlineOk dtypes.ConsiderOfflineStorageDealsConfigFunc,
	verifiedOk dtypes.ConsiderVerifiedStorageDealsConfigFunc,
	unverifiedOk dtypes.ConsiderUnverifiedStorageDealsConfigFunc,
	blocklistFunc dtypes.StorageDealPieceCidBlocklistConfigFunc,
	expectedSealTimeFunc dtypes.GetExpectedSealDurationFunc,
	startDelay dtypes.GetMaxDealStartDelayFunc,
	spn storagemarket.StorageProviderNode) dtypes.StorageDealFilter {
	return func(onlineOk dtypes.ConsiderOnlineStorageDealsConfigFunc,
		offlineOk dtypes.ConsiderOfflineStorageDealsConfigFunc,
		verifiedOk dtypes.ConsiderVerifiedStorageDealsConfigFunc,
		unverifiedOk dtypes.ConsiderUnverifiedStorageDealsConfigFunc,
		blocklistFunc dtypes.StorageDealPieceCidBlocklistConfigFunc,
		expectedSealTimeFunc dtypes.GetExpectedSealDurationFunc,
		startDelay dtypes.GetMaxDealStartDelayFunc,
		spn storagemarket.StorageProviderNode) dtypes.StorageDealFilter {

		return func(ctx context.Context, deal storagemarket.MinerDeal) (bool, string, error) {
			b, err := onlineOk()
			if err != nil {
				return false, "miner error", err
			}

			if deal.Ref != nil && deal.Ref.TransferType != storagemarket.TTManual && !b {
				log.Warnf("online storage deal consideration disabled; rejecting storage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not considering online storage deals", nil
			}

			b, err = offlineOk()
			if err != nil {
				return false, "miner error", err
			}

			if deal.Ref != nil && deal.Ref.TransferType == storagemarket.TTManual && !b {
				log.Warnf("offline storage deal consideration disabled; rejecting storage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not accepting offline storage deals", nil
			}

			b, err = verifiedOk()
			if err != nil {
				return false, "miner error", err
			}

			if deal.Proposal.VerifiedDeal && !b {
				log.Warnf("verified storage deal consideration disabled; rejecting storage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not accepting verified storage deals", nil
			}

			b, err = unverifiedOk()
			if err != nil {
				return false, "miner error", err
			}

			if !deal.Proposal.VerifiedDeal && !b {
				log.Warnf("unverified storage deal consideration disabled; rejecting storage deal proposal from client: %s", deal.Client.String())
				return false, "miner is not accepting unverified storage deals", nil
			}

			blocklist, err := blocklistFunc()
			if err != nil {
				return false, "miner error", err
			}

			for idx := range blocklist {
				if deal.Proposal.PieceCID.Equals(blocklist[idx]) {
					log.Warnf("piece CID in proposal %s is blocklisted; rejecting storage deal proposal from client: %s", deal.Proposal.PieceCID, deal.Client.String())
					return false, fmt.Sprintf("miner has blocklisted piece CID %s", deal.Proposal.PieceCID), nil
				}
			}

			sealDuration, err := expectedSealTimeFunc()
			if err != nil {
				return false, "miner error", err
			}

			sealEpochs := sealDuration / (time.Duration(build.BlockDelaySecs) * time.Second)
			_, ht, err := spn.GetChainHead(ctx)
			if err != nil {
				return false, "failed to get chain head", err
			}
			earliest := abi.ChainEpoch(sealEpochs) + ht
			if deal.Proposal.StartEpoch < earliest {
				log.Warnw("proposed deal would start before sealing can be completed; rejecting storage deal proposal from client", "piece_cid", deal.Proposal.PieceCID, "client", deal.Client.String(), "seal_duration", sealDuration, "earliest", earliest, "curepoch", ht)
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

func StorageProvider(minerAddress dtypes.MinerAddress,
	storedAsk *storedask.StoredAsk,
	h host.Host, ds dtypes.MetadataDS,
	mds dtypes.StagingMultiDstore,
	cfg *config.Market,
	pieceStore dtypes.ProviderPieceStore,
	dataTransfer dtypes.ProviderDataTransfer,
	spn storagemarket.StorageProviderNode,
	df dtypes.StorageDealFilter,
) (storagemarket.StorageProvider, error) {
	net := smnet.NewFromLibp2pHost(h)
	store, err := piecefilestore.NewLocalFileStore(piecefilestore.OsPath(cfg.HomeDir))
	if err != nil {
		return nil, err
	}

	opt := storageimpl.CustomDealDecisionLogic(storageimpl.DealDeciderFunc(df))

	return storageimpl.NewProvider(net, namespace.Wrap(ds, datastore.NewKey("/deals/provider")), store, mds, pieceStore, dataTransfer, spn, address.Address(minerAddress), storedAsk, opt)
}

func HandleDeals(mctx helpers.MetricsCtx, lc fx.Lifecycle, host host.Host, h storagemarket.StorageProvider, j journal.Journal) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	h.OnReady(marketevents.ReadyLogger("storage provider"))
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			h.SubscribeToEvents(marketevents.StorageProviderLogger)

			evtType := j.RegisterEventType("markets/storage/provider", "state_change")
			h.SubscribeToEvents(markets.StorageProviderJournaler(j, evtType))

			return h.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return h.Stop()
		},
	})
}

func HandleMigrateProviderFunds(lc fx.Lifecycle, ds dtypes.MetadataDS, node api.FullNode, minerAddress dtypes.MinerAddress) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			b, err := ds.Get(datastore.NewKey("/marketfunds/provider"))
			if err != nil {
				if xerrors.Is(err, datastore.ErrNotFound) {
					return nil
				}
				return err
			}

			var value abi.TokenAmount
			if err = value.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
				return err
			}
			ts, err := node.ChainHead(ctx)
			if err != nil {
				log.Errorf("provider funds migration - getting chain head: %v", err)
				return nil
			}

			mi, err := node.StateMinerInfo(ctx, address.Address(minerAddress), ts.Key())
			if err != nil {
				log.Errorf("provider funds migration - getting miner info %s: %v", minerAddress, err)
				return nil
			}

			_, err = node.MarketReserveFunds(ctx, mi.Worker, address.Address(minerAddress), value)
			if err != nil {
				log.Errorf("provider funds migration - reserving funds (wallet %s, addr %s, funds %d): %v",
					mi.Worker, minerAddress, value, err)
				return nil
			}

			return ds.Delete(datastore.NewKey("/marketfunds/provider"))
		},
	})
}

// RetrievalProvider creates a new retrieval provider attached to the provider blockstore
func RetrievalProvider(
	maddr dtypes.MinerAddress,
	adapter retrievalmarket.RetrievalProviderNode,
	netwk rmnet.RetrievalMarketNetwork,
	ds dtypes.MetadataDS,
	pieceStore dtypes.ProviderPieceStore,
	mds dtypes.StagingMultiDstore,
	dt dtypes.ProviderDataTransfer,
	pricingFnc dtypes.RetrievalPricingFunc,
	userFilter dtypes.RetrievalDealFilter,
) (retrievalmarket.RetrievalProvider, error) {
	opt := retrievalimpl.DealDeciderOpt(retrievalimpl.DealDecider(userFilter))
	return retrievalimpl.NewProvider(address.Address(maddr), adapter, netwk, pieceStore, mds, dt, namespace.Wrap(ds, datastore.NewKey("/retrievals/provider")),
		retrievalimpl.RetrievalPricingFunc(pricingFnc), opt)
}

func RetrievalDealFilter(userFilter dtypes.RetrievalDealFilter) func(onlineOk dtypes.ConsiderOnlineRetrievalDealsConfigFunc,
	offlineOk dtypes.ConsiderOfflineRetrievalDealsConfigFunc) dtypes.RetrievalDealFilter {
	return func(onlineOk dtypes.ConsiderOnlineRetrievalDealsConfigFunc,
		offlineOk dtypes.ConsiderOfflineRetrievalDealsConfigFunc) dtypes.RetrievalDealFilter {
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

func HandleRetrieval(host host.Host, lc fx.Lifecycle, m retrievalmarket.RetrievalProvider, j journal.Journal) {
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

func NewConsiderOnlineStorageDealsConfigFunc(cfg *config.Market) (dtypes.ConsiderOnlineStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOnlineStorageDeals, nil
	}, nil
}

func NewSetConsideringOnlineStorageDealsFunc(cfg *config.Market) (dtypes.SetConsiderOnlineStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOnlineStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderOnlineRetrievalDealsConfigFunc(cfg *config.Market) (dtypes.ConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOnlineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOnlineRetrievalDealsConfigFunc(cfg *config.Market) (dtypes.SetConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOnlineRetrievalDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewStorageDealPieceCidBlocklistConfigFunc(cfg *config.Market) (dtypes.StorageDealPieceCidBlocklistConfigFunc, error) {
	return func() (out []cid.Cid, err error) {
		return cfg.PieceCidBlocklist, nil
	}, nil
}

func NewSetStorageDealPieceCidBlocklistConfigFunc(cfg *config.Market) (dtypes.SetStorageDealPieceCidBlocklistConfigFunc, error) {
	return func(blocklist []cid.Cid) (err error) {
		cfg.PieceCidBlocklist = blocklist
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineStorageDealsConfigFunc(cfg *config.Market) (dtypes.ConsiderOfflineStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOfflineStorageDeals, nil
	}, nil
}

func NewSetConsideringOfflineStorageDealsFunc(cfg *config.Market) (dtypes.SetConsiderOfflineStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOfflineStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineRetrievalDealsConfigFunc(cfg *config.Market) (dtypes.ConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOfflineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOfflineRetrievalDealsConfigFunc(cfg *config.Market) (dtypes.SetConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOfflineRetrievalDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderVerifiedStorageDealsConfigFunc(cfg *config.Market) (dtypes.ConsiderVerifiedStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderVerifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringVerifiedStorageDealsFunc(cfg *config.Market) (dtypes.SetConsiderVerifiedStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderVerifiedStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewConsiderUnverifiedStorageDealsConfigFunc(cfg *config.Market) (dtypes.ConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderUnverifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringUnverifiedStorageDealsFunc(cfg *config.Market) (dtypes.SetConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderUnverifiedStorageDeals = b
		return config.SaveConfig(cfg)
	}, nil
}

func NewSetExpectedSealDurationFunc(cfg *config.Market) (dtypes.SetExpectedSealDurationFunc, error) {
	return func(delay time.Duration) (err error) {
		cfg.ExpectedSealDuration = config.Duration(delay)
		return config.SaveConfig(cfg)
	}, nil
}

func NewGetExpectedSealDurationFunc(cfg *config.Market) (dtypes.GetExpectedSealDurationFunc, error) {
	return func() (out time.Duration, err error) {
		return time.Duration(cfg.ExpectedSealDuration), nil
	}, nil
}

func NewSetMaxDealStartDelayFunc(cfg *config.Market) (dtypes.SetMaxDealStartDelayFunc, error) {
	return func(delay time.Duration) (err error) {
		cfg.MaxDealStartDelay = config.Duration(delay)
		return config.SaveConfig(cfg)
	}, nil
}

func NewGetMaxDealStartDelayFunc(cfg *config.Market) (dtypes.GetMaxDealStartDelayFunc, error) {
	return func() (out time.Duration, err error) {
		return time.Duration(cfg.MaxDealStartDelay), nil
	}, nil
}

// StagingDAG is a DAGService for the StagingBlockstore
func StagingDAG(mctx helpers.MetricsCtx, lc fx.Lifecycle, ibs dtypes.StagingBlockstore, rt routing.Routing, h host.Host) (dtypes.StagingDAG, error) {

	bitswapNetwork := network.NewFromIpfsHost(h, rt)
	bitswapOptions := []bitswap.Option{bitswap.ProvideEnabled(false)}
	exch := bitswap.New(mctx, bitswapNetwork, ibs, bitswapOptions...)

	bsvc := blockservice.New(ibs, exch)
	dag := merkledag.NewDAGService(bsvc)

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			// blockservice closes the exchange
			return bsvc.Close()
		},
	})

	return dag, nil
}

// StagingGraphsync creates a graphsync instance which reads and writes blocks
// to the StagingBlockstore
func StagingGraphsync(parallelTransfers uint64) func(mctx helpers.MetricsCtx, lc fx.Lifecycle, ibs dtypes.StagingBlockstore, h host.Host) dtypes.StagingGraphsync {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, ibs dtypes.StagingBlockstore, h host.Host) dtypes.StagingGraphsync {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		loader := storeutil.LoaderForBlockstore(ibs)
		storer := storeutil.StorerForBlockstore(ibs)
		gs := graphsync.New(helpers.LifecycleCtx(mctx, lc), graphsyncNetwork, loader, storer, graphsync.RejectAllRequestsByDefault(), graphsync.MaxInProgressRequests(parallelTransfers))

		return gs
	}
}

// NewProviderPieceStore creates a statestore for storing metadata about pieces
// shared by the storage and retrieval providers
func NewProviderPieceStore(lc fx.Lifecycle, ds dtypes.MetadataDS) (dtypes.ProviderPieceStore, error) {
	ps, err := piecestoreimpl.NewPieceStore(namespace.Wrap(ds, datastore.NewKey("/storagemarket")))
	if err != nil {
		return nil, err
	}
	ps.OnReady(marketevents.ReadyLogger("piecestore"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return ps.Start(ctx)
		},
	})
	return ps, nil
}
