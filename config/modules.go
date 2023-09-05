package config

import (
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mitchellh/go-homedir"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/filestore"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	vsTypes "github.com/filecoin-project/venus/venus-shared/types"
)

func NewConsiderOnlineStorageDealsConfigFunc(cfg *MarketConfig) (ConsiderOnlineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address) (bool, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, err
		}
		return pCfg.ConsiderOnlineStorageDeals, nil
	}, nil
}

func NewSetConsideringOnlineStorageDealsFunc(cfg *MarketConfig) (SetConsiderOnlineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		// mAddr==Undef,update global; otherwise, if exist, update, else create with global
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, err
		}
		return pCfg.ConsiderOnlineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOnlineRetrievalDealsConfigFunc(cfg *MarketConfig) (SetConsiderOnlineRetrievalDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return nil, err
		}
		return pCfg.PieceCidBlocklist, nil
	}, nil
}

func NewSetStorageDealPieceCidBlocklistConfigFunc(cfg *MarketConfig) (SetStorageDealPieceCidBlocklistConfigFunc, error) {
	return func(mAddr address.Address, blocklist []cid.Cid) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, err
		}
		return pCfg.ConsiderOfflineStorageDeals, nil
	}, nil
}

func NewSetConsideringOfflineStorageDealsFunc(cfg *MarketConfig) (SetConsiderOfflineStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, err
		}
		return pCfg.ConsiderOfflineRetrievalDeals, nil
	}, nil
}

func NewSetConsiderOfflineRetrievalDealsConfigFunc(cfg *MarketConfig) (SetConsiderOfflineRetrievalDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) (err error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, err
		}
		return pCfg.ConsiderVerifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringVerifiedStorageDealsFunc(cfg *MarketConfig) (SetConsiderVerifiedStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, err
		}
		return pCfg.ConsiderUnverifiedStorageDeals, nil
	}, nil
}

func NewSetConsideringUnverifiedStorageDealsFunc(cfg *MarketConfig) (SetConsiderUnverifiedStorageDealsConfigFunc, error) {
	return func(mAddr address.Address, b bool) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return 0, err
		}
		return time.Duration(pCfg.MaxDealStartDelay), nil
	}, nil
}

func NewSetMaxDealStartDelayFunc(cfg *MarketConfig) (SetMaxDealStartDelayFunc, error) {
	return func(mAddr address.Address, delay time.Duration) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
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
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return 0, err
		}
		return time.Duration(pCfg.ExpectedSealDuration), nil
	}, nil
}

func NewSetExpectedSealDurationFunc(cfg *MarketConfig) (SetExpectedSealDurationFunc, error) {
	return func(mAddr address.Address, delay time.Duration) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.ExpectedSealDuration = Duration(delay)
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewTransferPathFunc(cfg *MarketConfig) (TransferPathFunc, error) {
	return func(mAddr address.Address) (string, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return "", err
		}
		return pCfg.TransferPath, nil
	}, nil
}

func NewSetTransferPathFunc(cfg *MarketConfig) (SetTransferPathFunc, error) {
	return func(mAddr address.Address, path string) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.TransferPath = path
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewTransferFileStoreConfigFunc(cfg *MarketConfig) (TransferFileStoreConfigFunc, error) {
	return func(mAddr address.Address) (filestore.FileStore, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return nil, err
		}
		transferPath := pCfg.TransferPath
		if len(transferPath) == 0 {
			transferPath = cfg.MustHomePath()
		} else {
			transferPath, err = homedir.Expand(transferPath)
			if err != nil {
				return nil, err
			}
		}

		store, err := filestore.NewLocalFileStore(filestore.OsPath(transferPath))
		if err != nil {
			return nil, err
		}
		return store, nil
	}, nil
}

func NewPublishMsgPeriodConfigFunc(cfg *MarketConfig) (PublishMsgPeriodConfigFunc, error) {
	return func(mAddr address.Address) (time.Duration, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return 0, err
		}
		return time.Duration(pCfg.PublishMsgPeriod), nil
	}, nil
}

func NewSetPublishMsgPeriodConfigFunc(cfg *MarketConfig) (SetPublishMsgPeriodConfigFunc, error) {
	return func(mAddr address.Address, d time.Duration) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.PublishMsgPeriod = Duration(d)
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewMaxDealsPerPublishMsgFunc(cfg *MarketConfig) (MaxDealsPerPublishMsgFunc, error) {
	return func(mAddr address.Address) (uint64, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return 0, err
		}
		return pCfg.MaxDealsPerPublishMsg, err
	}, nil
}

func NewSetMaxDealsPerPublishMsgFunc(cfg *MarketConfig) (SetMaxDealsPerPublishMsgFunc, error) {
	return func(mAddr address.Address, nums uint64) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.MaxDealsPerPublishMsg = nums
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewMaxProviderCollateralMultiplierFunc(cfg *MarketConfig) (MaxProviderCollateralMultiplierFunc, error) {
	return func(mAddr address.Address) (uint64, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return 0, err
		}
		return pCfg.MaxProviderCollateralMultiplier, nil
	}, nil
}

func NewSetMaxProviderCollateralMultiplierFunc(cfg *MarketConfig) (SetMaxProviderCollateralMultiplierFunc, error) {
	return func(mAddr address.Address, c uint64) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.MaxProviderCollateralMultiplier = c
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewMaxPublishDealsFeeFunc(cfg *MarketConfig) (MaxPublishDealsFeeFunc, error) {
	return func(mAddr address.Address) (vsTypes.FIL, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return vsTypes.FIL{}, err
		}
		return pCfg.MaxPublishDealsFee, nil
	}, nil
}

func NewSetMaxPublishDealsFeeFunc(cfg *MarketConfig) (SetMaxPublishDealsFeeFunc, error) {
	return func(mAddr address.Address, f vsTypes.FIL) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.MaxPublishDealsFee = f
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

func NewMaxMarketBalanceAddFeeFunc(cfg *MarketConfig) (MaxMarketBalanceAddFeeFunc, error) {
	return func(mAddr address.Address) (vsTypes.FIL, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return vsTypes.FIL{}, err
		}
		return pCfg.MaxMarketBalanceAddFee, nil
	}, nil
}

func NewSetMaxMarketBalanceAddFeeFunc(cfg *MarketConfig) (SetMaxMarketBalanceAddFeeFunc, error) {
	return func(mAddr address.Address, f vsTypes.FIL) error {
		pCfg, err := cfg.MinerProviderConfig(mAddr, false)
		if err != nil {
			return err
		}
		if pCfg == nil {
			pCfg = defaultProviderConfig()
		}
		pCfg.MaxMarketBalanceAddFee = f
		cfg.SetMinerProviderConfig(mAddr, pCfg)
		return SaveConfig(cfg)
	}, nil
}

var ConfigServerOpts = func(cfg *MarketConfig) builder.Option {
	return builder.Options(
		builder.Override(new(*MarketConfig), cfg),
		builder.Override(new(*HomeDir), cfg.HomePath),
		builder.Override(new(IHome), cfg),
		builder.Override(new(Node), cfg.GetNode()),
		builder.Override(new(*Messager), cfg.GetMessager),
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

		builder.Override(new(TransferPathFunc), NewTransferPathFunc),
		builder.Override(new(SetTransferPathFunc), NewSetTransferPathFunc),
		builder.Override(new(TransferFileStoreConfigFunc), NewTransferFileStoreConfigFunc),

		builder.Override(new(PublishMsgPeriodConfigFunc), NewPublishMsgPeriodConfigFunc),
		builder.Override(new(SetPublishMsgPeriodConfigFunc), NewSetPublishMsgPeriodConfigFunc),
		builder.Override(new(MaxDealsPerPublishMsgFunc), NewMaxDealsPerPublishMsgFunc),
		builder.Override(new(SetMaxDealsPerPublishMsgFunc), NewSetMaxDealsPerPublishMsgFunc),
		builder.Override(new(MaxProviderCollateralMultiplierFunc), NewMaxProviderCollateralMultiplierFunc),
		builder.Override(new(SetMaxProviderCollateralMultiplierFunc), NewSetMaxProviderCollateralMultiplierFunc),

		builder.Override(new(MaxPublishDealsFeeFunc), NewMaxPublishDealsFeeFunc),
		builder.Override(new(SetMaxPublishDealsFeeFunc), NewSetMaxPublishDealsFeeFunc),
		builder.Override(new(MaxMarketBalanceAddFeeFunc), NewMaxMarketBalanceAddFeeFunc),
		builder.Override(new(SetMaxMarketBalanceAddFeeFunc), NewSetMaxMarketBalanceAddFeeFunc),
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
