package retrievalprovider

import (
	"context"

	"go.uber.org/fx"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/filecoin-project/go-address"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"

	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/dealfilter"
	_ "github.com/filecoin-project/venus-market/v2/network"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

var HandleRetrievalKey = builder.NextInvoke()

func RetrievalDealFilter(userFilter config.RetrievalDealFilter) func(onlineOk config.ConsiderOnlineRetrievalDealsConfigFunc,
	offlineOk config.ConsiderOfflineRetrievalDealsConfigFunc) config.RetrievalDealFilter {
	return func(onlineOk config.ConsiderOnlineRetrievalDealsConfigFunc,
		offlineOk config.ConsiderOfflineRetrievalDealsConfigFunc,
	) config.RetrievalDealFilter {
		return func(ctx context.Context, mAddr address.Address, state types.ProviderDealState) (bool, string, error) {
			b, err := onlineOk(mAddr)
			if err != nil {
				return false, "miner error", err
			}

			if !b {
				log.Warn("online retrieval deal consideration disabled; rejecting retrieval deal proposal from client")
				return false, "miner is not accepting online retrieval deals", nil
			}

			b, err = offlineOk(mAddr)
			if err != nil {
				return false, "miner error", err
			}
			if !b {
				log.Info("offline retrieval has not been implemented yet")
			}

			// user never will be nil?
			return userFilter(ctx, mAddr, state)
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

func RetrievalNetwork(h host.Host) rmnet.RetrievalMarketNetwork {
	return rmnet.NewFromLibp2pHost(h)
}

var RetrievalProviderOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		// Markets (retrieval)
		builder.Override(new(rmnet.RetrievalMarketNetwork), RetrievalNetwork),
		builder.Override(new(IRetrievalProvider), NewProvider), // save to metadata /retrievals/provider
		builder.Override(HandleRetrievalKey, HandleRetrieval),
		builder.Override(new(config.RetrievalDealFilter), RetrievalDealFilter(dealfilter.CliRetrievalDealFilter(cfg))),
	)
}
