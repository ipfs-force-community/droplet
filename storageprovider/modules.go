package storageprovider

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"
	dtimpl "github.com/filecoin-project/go-data-transfer/v2/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/v2/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/v2/transport/graphsync"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/dealfilter"
	"github.com/ipfs-force-community/droplet/v2/models/badger"
	"github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/utils"

	"github.com/filecoin-project/venus/pkg/constants"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
)

var (
	HandleDealsKey   = builder.NextInvoke()
	StartDealTracker = builder.NextInvoke()
)

func HandleDeals(mctx metrics.MetricsCtx, lc fx.Lifecycle, h StorageProvider, j journal.Journal) {
	ctx := metrics.LifecycleCtx(mctx, lc)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return h.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return h.Stop()
		},
	})
}

// NewProviderDAGServiceDataTransfer returns a data transfer manager that just
// uses the provider's Staging DAG service for transfers
func NewProviderDAGServiceDataTransfer(lc fx.Lifecycle, dagDs badger.DagTransferDS, h host.Host, homeDir *config.HomeDir, gs network.StagingGraphsync) (network.ProviderDataTransfer, error) {
	net := dtnet.NewFromLibp2pHost(h)
	transport := dtgstransport.NewTransport(h.ID(), gs)

	dt, err := dtimpl.NewDataTransfer(dagDs, net, transport)
	if err != nil {
		return nil, err
	}

	dt.OnReady(utils.ReadyLogger("provider data transfer"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			dt.SubscribeToEvents(utils.DataTransferLogger)
			return dt.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return dt.Stop(ctx)
		},
	})
	return dt, nil
}

func BasicDealFilter(user config.StorageDealFilter) func(onlineOk config.ConsiderOnlineStorageDealsConfigFunc,
	offlineOk config.ConsiderOfflineStorageDealsConfigFunc,
	verifiedOk config.ConsiderVerifiedStorageDealsConfigFunc,
	unverifiedOk config.ConsiderUnverifiedStorageDealsConfigFunc,
	blocklistFunc config.StorageDealPieceCidBlocklistConfigFunc,
	expectedSealTimeFunc config.GetExpectedSealDurationFunc,
	startDelay config.GetMaxDealStartDelayFunc,
	spn StorageProviderNode) config.StorageDealFilter {
	return func(onlineOk config.ConsiderOnlineStorageDealsConfigFunc,
		offlineOk config.ConsiderOfflineStorageDealsConfigFunc,
		verifiedOk config.ConsiderVerifiedStorageDealsConfigFunc,
		unverifiedOk config.ConsiderUnverifiedStorageDealsConfigFunc,
		blocklistFunc config.StorageDealPieceCidBlocklistConfigFunc,
		expectedSealTimeFunc config.GetExpectedSealDurationFunc,
		startDelay config.GetMaxDealStartDelayFunc,
		spn StorageProviderNode,
	) config.StorageDealFilter {
		return func(ctx context.Context, mAddr address.Address, deal *types2.DealParams) (bool, string, error) {
			proposal := deal.ClientDealProposal.Proposal
			client := deal.ClientDealProposal.Proposal.Client

			b, err := onlineOk(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			if !deal.IsOffline && !b {
				log.Warnf("online piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", client.String())
				return false, "miner is not considering online piecestorage deals", nil
			}

			b, err = offlineOk(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			if deal.IsOffline && !b {
				log.Warnf("offline piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", client.String())
				return false, "miner is not accepting offline piecestorage deals", nil
			}

			b, err = verifiedOk(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			if proposal.VerifiedDeal && !b {
				log.Warnf("verified piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", client.String())
				return false, "miner is not accepting verified piecestorage deals", nil
			}

			b, err = unverifiedOk(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			if !proposal.VerifiedDeal && !b {
				log.Warnf("unverified piecestorage deal consideration disabled; rejecting piecestorage deal proposal from client: %s", client.String())
				return false, "miner is not accepting unverified piecestorage deals", nil
			}

			blocklist, err := blocklistFunc(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			for idx := range blocklist {
				if proposal.PieceCID.Equals(blocklist[idx]) {
					log.Warnf("piece CID in proposal %s is blocklisted; rejecting piecestorage deal proposal from client: %s", proposal.PieceCID, client.String())
					return false, fmt.Sprintf("miner has blocklisted piece CID %s", proposal.PieceCID), nil
				}
			}

			sealDuration, err := expectedSealTimeFunc(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			sealEpochs := sealDuration / (time.Duration(constants.MainNetBlockDelaySecs) * time.Second)
			_, ht, err := spn.GetChainHead(ctx)
			if err != nil {
				return false, "failed to get chain head", err
			}
			earliest := abi.ChainEpoch(sealEpochs) + ht
			if proposal.StartEpoch < earliest {
				log.Warnw("proposed deal would start before sealing can be completed; rejecting piecestorage deal proposal from client", "piece_cid", proposal.PieceCID, "client", client.String(), "seal_duration", sealDuration, "earliest", earliest, "curepoch", ht)
				return false, fmt.Sprintf("cannot seal a sector before %s", proposal.StartEpoch), nil
			}

			sd, err := startDelay(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			// Reject if it's more than 7 days in the future
			// TODO: read from cfg how to get block delay
			maxStartEpoch := earliest + abi.ChainEpoch(uint64(sd.Seconds())/constants.MainNetBlockDelaySecs)
			if proposal.StartEpoch > maxStartEpoch {
				return false, fmt.Sprintf("deal start epoch is too far in the future: %s > %s", proposal.StartEpoch, maxStartEpoch), nil
			}

			// user never will be nil?
			return user(ctx, mAddr, deal)
		}
	}
}

var StorageProviderOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(IStorageAsk), NewStorageAsk),
		builder.Override(new(network.ProviderDataTransfer), NewProviderDAGServiceDataTransfer), // save to metadata /datatransfer/provider/transfers
		//   save to metadata /deals/provider/piecestorage-ask/latest
		builder.Override(new(StorageProvider), NewStorageProvider),
		builder.Override(new(*DealPublisher), NewDealPublisherWrapper(cfg)),
		builder.Override(HandleDealsKey, HandleDeals),
		builder.Override(new(config.StorageDealFilter), BasicDealFilter(dealfilter.CliStorageDealFilter(cfg))),
		builder.Override(new(StorageProviderNode), NewProviderNodeAdapter(cfg)),
		builder.Override(new(DealAssiger), NewDealAssigner),
		builder.Override(StartDealTracker, NewDealTracker),
		builder.Override(new(*EventPublishAdapter), NewEventPublishAdapter),
		builder.Override(new(*DirectDealProvider), NewDirectDealProvider),
	)
}

var StorageClientOpts = builder.Options(
	builder.Override(new(storagemarket.StorageClientNode), NewClientNodeAdapter),
)
