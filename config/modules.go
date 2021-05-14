package config

import (
	"io/ioutil"
	"os"

	"github.com/pelletier/go-toml"
)

func ConfigExit(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func ReadConfig(path string) (*Config, error) {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := new(Config)
	if err := toml.Unmarshal(configBytes, config); err != nil {
		return nil, err
	}
	return config, nil
}

func WriteConfig(path string, cfg *Config) error {
	cfgBytes, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, cfgBytes, 0666)
}
