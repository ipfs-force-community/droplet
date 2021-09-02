package config

import (
	"github.com/filecoin-project/venus-market/builder"
)

var ConfigServerOpts = func(cfg *MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(*MarketConfig), cfg),
		builder.Override(new(*HomeDir), cfg.HomePath),
		builder.Override(new(IHome), cfg),
		builder.Override(new(*Node), &cfg.Node),
		builder.Override(new(*Messager), &cfg.Messager),
		builder.Override(new(*Signer), &cfg.Signer),
		builder.Override(new(*Sealer), &cfg.Sealer),
		builder.Override(new(*Libp2p), &cfg.Libp2p),
		builder.Override(new(*PieceStorage), &cfg.PieceStorage),
		builder.Override(new(*DAGStoreConfig), &cfg.DAGStore),
	)
}

var ConfigClientOpts = func(cfg *MarketClientConfig) builder.Option {
	return builder.Options(
		builder.Override(new(*MarketClientConfig), cfg),
		builder.Override(new(IHome), cfg),
		builder.Override(new(*HomeDir), cfg.HomePath),
		builder.Override(new(*Node), &cfg.Node),
		builder.Override(new(*Libp2p), &cfg.Libp2p),
		builder.Override(new(*Signer), &cfg.Signer),
		builder.Override(new(*Messager), &cfg.Messager),
	)
}
