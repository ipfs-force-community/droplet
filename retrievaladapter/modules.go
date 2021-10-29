package retrievaladapter

import (
	"context"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus-market/models/repo"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/dealfilter"
	"github.com/filecoin-project/venus-market/journal"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"
)

var (
	HandleRetrievalKey builder.Invoke = builder.NextInvoke()
)

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

func RetrievalProvider(node retrievalmarket.RetrievalProviderNode,
	network rmnet.RetrievalMarketNetwork,
	dagStore stores.DAGStoreWrapper,
	dataTransfer datatransfer.Manager,
	retrievalPricingFunc retrievalimpl.RetrievalPricingFunc,

	askRepo repo.IRetrievalAskRepo,
	storageDealsRepo repo.StorageDealRepo,
	reterivalDealRepo repo.IRetrievalDealRepo,
	cidInfoRepo repo.ICidInfoRepo) (IRetrievalProvider, error) {
	return NewProvider(node, network, dagStore, dataTransfer, retrievalPricingFunc, askRepo, storageDealsRepo, reterivalDealRepo, cidInfoRepo)
}

func HandleRetrieval(host host.Host,
	lc fx.Lifecycle,
	m RetrievalProviderV2,
	j journal.Journal,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return m.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return m.Stop()
		},
	})
}

// RetrievalPricingFunc configures the pricing function to use for retrieval deals.
func RetrievalPricingFunc(cfg *config.MarketConfig) func(_ config.ConsiderOnlineRetrievalDealsConfigFunc,
	_ config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalPricingFunc {

	return func(_ config.ConsiderOnlineRetrievalDealsConfigFunc,
		_ config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalPricingFunc {
		if cfg.RetrievalPricing.Strategy == config.RetrievalPricingExternalMode {
			return ExternalRetrievalPricingFunc(cfg.RetrievalPricing.External.Path)
		}

		return retrievalimpl.DefaultPricingFunc(cfg.RetrievalPricing.Default.VerifiedDealsFreeTransfer)
	}
}

func RetrievalNetwork(h host.Host) rmnet.RetrievalMarketNetwork {
	return rmnet.NewFromLibp2pHost(h)
}

var RetrievalProviderOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(

		builder.Override(new(rmnet.RetrievalMarketNetwork), RetrievalNetwork),
		// Markets (retrieval deps)
		builder.Override(new(config.RetrievalPricingFunc), RetrievalPricingFunc(cfg)),
		// Markets (retrieval)
		builder.Override(new(retrievalmarket.RetrievalProviderNode), NewRetrievalProviderNode),
		builder.Override(new(rmnet.RetrievalMarketNetwork), RetrievalNetwork),
		builder.Override(new(IRetrievalProvider), RetrievalProvider), //save to metadata /retrievals/provider
		builder.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(nil)),
		builder.Override(HandleRetrievalKey, HandleRetrieval),
		builder.If(cfg.RetrievalFilter != "",
			builder.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(dealfilter.CliRetrievalDealFilter(cfg.RetrievalFilter))),
		),
	)
}
