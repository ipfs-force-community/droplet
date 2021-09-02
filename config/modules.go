package config

import (
	"github.com/filecoin-project/venus-market/builder"
)

var ConfigOpts = func(cfg *MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(HomeDir), cfg.HomePath),
		builder.Override(new(*MarketConfig), cfg),
		builder.Override(new(*Node), &cfg.Node),
		builder.Override(new(*Messager), &cfg.Messager),
		builder.Override(new(*Gateway), &cfg.Gateway),
		builder.Override(new(*Sealer), &cfg.Sealer),
		builder.Override(new(*PieceStorage), &cfg.PieceStorage),
		builder.Override(new(*DAGStoreConfig), &cfg.DAGStore),
	)
}
