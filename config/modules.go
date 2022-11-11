package config

import (
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mitchellh/go-homedir"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/filestore"

	"github.com/ipfs-force-community/venus-common-utils/builder"
)

func NewConsiderOnlineStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderOnlineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.ConsiderOnlineStorageDeals, nil
	}, nil
}

func NewSetConsideringOnlineStorageDealsFunc(cfg *MarketConfig) (SetConsiderOnlineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ConsiderOnlineStorageDeals = b
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderOnlineRetrievalDealsConfigFunc(cfg *MarketConfig) (ConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.ConsiderOnlineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOnlineRetrievalDealsConfigFunc(cfg *MarketConfig) (SetConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ConsiderOnlineRetrievalDeals = b
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewStorageDealPieceCidBlocklistConfigFunc(cfg *MarketConfig) (StorageDealPieceCidBlocklistConfigFunc, error) {
	return func(mAddr address.Address) ([]cid.Cid, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.PieceCidBlocklist, nil
	}, nil
}

func NewSetStorageDealPieceCidBlocklistConfigFunc(cfg *MarketConfig) (SetStorageDealPieceCidBlocklistConfigFunc, error) {
	return func(mAddr address.Address, blocklist []cid.Cid) (err error) {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.PieceCidBlocklist = blocklist
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderOfflineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.ConsiderOfflineStorageDeals, nil
	}, nil
}

func NewSetConsideringOfflineStorageDealsFunc(cfg *MarketConfig) (SetConsiderOfflineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ConsiderOfflineStorageDeals = b
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderOfflineRetrievalDealsConfigFunc(cfg *MarketConfig) (ConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.ConsiderOfflineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOfflineRetrievalDealsConfigFunc(cfg *MarketConfig) (SetConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) (err error) {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ConsiderOfflineRetrievalDeals = b
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderVerifiedStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderVerifiedStorageDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.ConsiderVerifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringVerifiedStorageDealsFunc(cfg *MarketConfig) (SetConsiderVerifiedStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ConsiderVerifiedStorageDeals = b
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewConsiderUnverifiedStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return pCfg.ConsiderUnverifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringUnverifiedStorageDealsFunc(cfg *MarketConfig) (SetConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ConsiderUnverifiedStorageDeals = b
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewGetMaxDealStartDelayFunc(cfg *MarketConfig) (GetMaxDealStartDelayFunc, error) {
	return func(mAddr address.Address) (time.Duration, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return time.Duration(pCfg.MaxDealStartDelay), nil
	}, nil
}

func NewSetMaxDealStartDelayFunc(cfg *MarketConfig) (SetMaxDealStartDelayFunc, error) {
	return func(mAddr address.Address, delay time.Duration) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.MaxDealStartDelay = Duration(delay)
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewGetExpectedSealDurationFunc(cfg *MarketConfig) (GetExpectedSealDurationFunc, error) {
	return func(mAddr address.Address) (time.Duration, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		return time.Duration(pCfg.ExpectedSealDuration), nil
	}, nil
}

func NewSetExpectedSealDurationFunc(cfg *MarketConfig) (SetExpectedSealDurationFunc, error) {
	return func(mAddr address.Address, delay time.Duration) error {
		pCfg := cfg.MinerProviderConfig(mAddr, false)
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ExpectedSealDuration = Duration(delay)
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewTransferFileStoreConfigFunc(cfg *MarketConfig) (TransferFileStoreConfigFunc, error) {
	return func(mAddr address.Address) (store filestore.FileStore, err error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		transferPath := pCfg.TransferPath
		if len(transferPath) == 0 {
			transferPath = cfg.MustHomePath()
		} else {
			transferPath, err = homedir.Expand(transferPath)
			if err != nil {
				return nil, err
			}
		}

		store, err = filestore.NewLocalFileStore(filestore.OsPath(transferPath))
		if err != nil {
			return nil, err
		}
		return store, nil
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

		builder.Override(new(TransferFileStoreConfigFunc), NewTransferFileStoreConfigFunc),
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
