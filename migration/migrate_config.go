package migration

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus-market/v2/config"
)

const (
	marketLatestVersion = 1
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
