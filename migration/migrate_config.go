package migration

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/rpc"
)

const (
	marketLatestVersion       = 1
	marketClientLatestVersion = 1
)

var migrateConfigLog = logging.Logger("config_migrate")

type marketUpgradeFunc func(cfg *config.MarketConfig) error

type marketVersionInfo struct {
	version int
	upgrade marketUpgradeFunc
}

var marketVersionMap = []marketVersionInfo{
	{version: 1, upgrade: marketVersion1Upgrade},
}

func TryToMigrateMarketConfig(cfg *config.MarketConfig) error {
	localVersion, err := loadVersion(cfg)
	if err != nil {
		if xerrors.Is(err, os.ErrNotExist) {
			localVersion = marketLatestVersion - 1
		} else {
			return err
		}
	}

	for _, up := range marketVersionMap {
		if up.version > localVersion {
			err = up.upgrade(cfg)
			if err != nil {
				return err
			}
			migrateConfigLog.Infof("success to upgrade version %d to version %d", localVersion, up.version)
			localVersion = up.version
		}
	}

	return nil
}

func marketVersion1Upgrade(cfg *config.MarketConfig) error {
	cfg.TransportConfig = config.DefaultMarketConfig.TransportConfig
	if err := config.SaveConfig(cfg); err != nil {
		return err
	}

	return saveVersion(1, cfg)
}

////// migrate market client config  //////

type marketClientUpgradeFunc func(cfg *config.MarketClientConfig) error

type marketClientVersionInfo struct {
	version int
	upgrade marketClientUpgradeFunc
}

var marketClientVersionMap = []marketClientVersionInfo{
	{version: 1, upgrade: marketClientVersion1Upgrade},
}

func marketClientVersion1Upgrade(cfg *config.MarketClientConfig) error {
	cfg.Market.Token = ""
	cfg.Market.Url = ""
	cfg.DealDir = rpc.DefDealsDir
	if err := config.SaveConfig(cfg); err != nil {
		return err
	}

	return saveVersion(1, cfg)
}

func TryToMigrateClientConfig(cfg *config.MarketClientConfig) error {
	localVersion, err := loadVersion(cfg)
	if err != nil {
		if xerrors.Is(err, os.ErrNotExist) {
			localVersion = marketClientLatestVersion - 1
		} else {
			return err
		}
	}

	for _, up := range marketClientVersionMap {
		if up.version > localVersion {
			err = up.upgrade(cfg)
			if err != nil {
				return err
			}
			migrateConfigLog.Infof("success to upgrade version %d to version %d", localVersion, up.version)
			localVersion = up.version
		}
	}

	return nil
}

//// util ////

func loadVersion(cfg config.IHome) (int, error) {
	vpath, err := cfg.VersionPath()
	if err != nil {
		return 0, err
	}
	b, err := ioutil.ReadFile(vpath)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(b))
}

func saveVersion(version int, cfg config.IHome) error {
	vpath, err := cfg.VersionPath()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(vpath, []byte(fmt.Sprintf("%v", version)), 0644)
}
