package config

import (
	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml"
	"io/ioutil"
	"os"
	"path"
)

func SaveConfig(cfg IHome) error {
	cfgBytes, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return err
	}

	_ = os.MkdirAll(path.Dir(cfgPath), os.ModePerm)
	return ioutil.WriteFile(cfgPath, cfgBytes, 0644)
}

func LoadConfig(cfgPath string, cfg IHome) error {
	homeDir, err := homedir.Expand(cfgPath)
	if err != nil {
		return err
	}

	cfgBytes, err := ioutil.ReadFile(homeDir)
	if err != nil {
		return err
	}
	return toml.Unmarshal(cfgBytes, cfg)
}
