package config

import (
	"time"

	"github.com/ipfs/go-cid"

	"github.com/ipfs-force-community/venus-common-utils/builder"
)

func NewConsiderOnlineStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderOnlineStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOnlineStorageDeals, nil
	}, nil
}

func NewSetConsideringOnlineStorageDealsFunc(cfg *MarketConfig) (SetConsiderOnlineStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOnlineStorageDeals = b
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderOnlineRetrievalDealsConfigFunc(cfg *MarketConfig) (ConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOnlineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOnlineRetrievalDealsConfigFunc(cfg *MarketConfig) (SetConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOnlineRetrievalDeals = b
		return SaveConfig(cfg)
	}, nil
}

func NewStorageDealPieceCidBlocklistConfigFunc(cfg *MarketConfig) (StorageDealPieceCidBlocklistConfigFunc, error) {
	return func() (out []cid.Cid, err error) {
		return cfg.PieceCidBlocklist, nil
	}, nil
}

func NewSetStorageDealPieceCidBlocklistConfigFunc(cfg *MarketConfig) (SetStorageDealPieceCidBlocklistConfigFunc, error) {
	return func(blocklist []cid.Cid) (err error) {
		cfg.PieceCidBlocklist = blocklist
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderOfflineStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOfflineStorageDeals, nil
	}, nil
}

func NewSetConsideringOfflineStorageDealsFunc(cfg *MarketConfig) (SetConsiderOfflineStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOfflineStorageDeals = b
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineRetrievalDealsConfigFunc(cfg *MarketConfig) (ConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderOfflineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOfflineRetrievalDealsConfigFunc(cfg *MarketConfig) (SetConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderOfflineRetrievalDeals = b
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderVerifiedStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderVerifiedStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderVerifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringVerifiedStorageDealsFunc(cfg *MarketConfig) (SetConsiderVerifiedStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderVerifiedStorageDeals = b
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderUnverifiedStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func() (out bool, err error) {
		return cfg.ConsiderUnverifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringUnverifiedStorageDealsFunc(cfg *MarketConfig) (SetConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func(b bool) (err error) {
		cfg.ConsiderUnverifiedStorageDeals = b
		return SaveConfig(cfg)
	}, nil
}

func NewSetExpectedSealDurationFunc(cfg *MarketConfig) (SetExpectedSealDurationFunc, error) {
	return func(delay time.Duration) (err error) {
		cfg.ExpectedSealDuration = Duration(delay)
		return SaveConfig(cfg)
	}, nil
}

func NewGetExpectedSealDurationFunc(cfg *MarketConfig) (GetExpectedSealDurationFunc, error) {
	return func() (out time.Duration, err error) {
		return time.Duration(cfg.ExpectedSealDuration), nil
	}, nil
}

func NewSetMaxDealStartDelayFunc(cfg *MarketConfig) (SetMaxDealStartDelayFunc, error) {
	return func(delay time.Duration) (err error) {
		cfg.MaxDealStartDelay = Duration(delay)
		return SaveConfig(cfg)
	}, nil
}

func NewGetMaxDealStartDelayFunc(cfg *MarketConfig) (GetMaxDealStartDelayFunc, error) {
	return func() (out time.Duration, err error) {
		return time.Duration(cfg.MaxDealStartDelay), nil
	}, nil
}

var ConfigServerOpts = func(cfg *MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(*MarketConfig), cfg),
		builder.Override(new(*HomeDir), cfg.HomePath),
		builder.Override(new(IHome), cfg),
		builder.Override(new(*Node), &cfg.Node),
		builder.Override(new(*Messager), &cfg.Messager),
		builder.Override(new(*Signer), &cfg.Signer),
		builder.Override(new(*Mysql), &cfg.Mysql),
		builder.Override(new(*Libp2p), &cfg.Libp2p),
		builder.Override(new(*PieceStorage), &cfg.PieceStorage),
		builder.Override(new(*DAGStoreConfig), &cfg.DAGStore),

		// Config (todo: get a real property system)
		builder.Override(new(ConsiderOnlineStorageDealsConfigFunc), NewConsiderOnlineStorageDealsConfigFunc),
		builder.Override(new(SetConsiderOnlineStorageDealsConfigFunc), NewSetConsideringOnlineStorageDealsFunc),
		builder.Override(new(ConsiderOnlineRetrievalDealsConfigFunc), NewConsiderOnlineRetrievalDealsConfigFunc),
		builder.Override(new(SetConsiderOnlineRetrievalDealsConfigFunc), NewSetConsiderOnlineRetrievalDealsConfigFunc),
		builder.Override(new(StorageDealPieceCidBlocklistConfigFunc), NewStorageDealPieceCidBlocklistConfigFunc),
		builder.Override(new(SetStorageDealPieceCidBlocklistConfigFunc), NewSetStorageDealPieceCidBlocklistConfigFunc),
		builder.Override(new(ConsiderOfflineStorageDealsConfigFunc), NewConsiderOfflineStorageDealsConfigFunc),
		builder.Override(new(SetConsiderOfflineStorageDealsConfigFunc), NewSetConsideringOfflineStorageDealsFunc),
		builder.Override(new(ConsiderOfflineRetrievalDealsConfigFunc), NewConsiderOfflineRetrievalDealsConfigFunc),
		builder.Override(new(SetConsiderOfflineRetrievalDealsConfigFunc), NewSetConsiderOfflineRetrievalDealsConfigFunc),
		builder.Override(new(ConsiderVerifiedStorageDealsConfigFunc), NewConsiderVerifiedStorageDealsConfigFunc),
		builder.Override(new(SetConsiderVerifiedStorageDealsConfigFunc), NewSetConsideringVerifiedStorageDealsFunc),
		builder.Override(new(ConsiderUnverifiedStorageDealsConfigFunc), NewConsiderUnverifiedStorageDealsConfigFunc),
		builder.Override(new(SetConsiderUnverifiedStorageDealsConfigFunc), NewSetConsideringUnverifiedStorageDealsFunc),
		builder.Override(new(SetExpectedSealDurationFunc), NewSetExpectedSealDurationFunc),
		builder.Override(new(GetExpectedSealDurationFunc), NewGetExpectedSealDurationFunc),
		builder.Override(new(SetMaxDealStartDelayFunc), NewSetMaxDealStartDelayFunc),
		builder.Override(new(GetMaxDealStartDelayFunc), NewGetMaxDealStartDelayFunc),
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
