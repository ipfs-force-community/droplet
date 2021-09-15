package main

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"time"
)

var (
	log = logging.Logger("modules")
)

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
