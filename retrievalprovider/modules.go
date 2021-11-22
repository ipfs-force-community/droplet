package retrievalprovider

import (
	"context"
	"github.com/filecoin-project/venus-market/types"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/dealfilter"
	_ "github.com/filecoin-project/venus-market/network"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"
)

var (
	HandleRetrievalKey = builder.NextInvoke()
)

func RetrievalDealFilter(userFilter config.RetrievalDealFilter) func(onlineOk config.ConsiderOnlineRetrievalDealsConfigFunc,
	offlineOk config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalDealFilter {
	return func(onlineOk config.ConsiderOnlineRetrievalDealsConfigFunc,
		offlineOk config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalDealFilter {
		return func(ctx context.Context, state types.ProviderDealState) (bool, string, error) {
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

func HandleRetrieval(
	lc fx.Lifecycle,
	m IRetrievalProvider,
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
	_ config.ConsiderOfflineRetrievalDealsConfigFunc) retrievalimpl.RetrievalPricingFunc {

	return func(_ config.ConsiderOnlineRetrievalDealsConfigFunc,
		_ config.ConsiderOfflineRetrievalDealsConfigFunc) retrievalimpl.RetrievalPricingFunc {
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
		builder.Override(new(retrievalimpl.RetrievalPricingFunc), RetrievalPricingFunc(cfg)),
		// Markets (retrieval)
		builder.Override(new(retrievalmarket.RetrievalProviderNode), NewRetrievalProviderNode),
		builder.Override(new(rmnet.RetrievalMarketNetwork), RetrievalNetwork),
		builder.Override(new(IAskHandler), NewAskHandler),
		builder.Override(new(IRetrievalProvider), NewProvider), // save to metadata /retrievals/provider
		builder.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(nil)),
		builder.Override(HandleRetrievalKey, HandleRetrieval),
		builder.If(cfg.RetrievalFilter != "",
			builder.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(dealfilter.CliRetrievalDealFilter(cfg.RetrievalFilter))),
		),
	)
}
