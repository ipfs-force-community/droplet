package config

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

func SaveConfig(cfg IHome) error {
	buf := new(bytes.Buffer)
	_, _ = buf.WriteString("# Default config:\n")
	e := toml.NewEncoder(buf)

	err := e.Encode(cfg)
	if err != nil {
		return err
	}
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return err
	}

	_ = os.MkdirAll(path.Dir(cfgPath), os.ModePerm)
	return ioutil.WriteFile(cfgPath, buf.Bytes(), 0644)
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
