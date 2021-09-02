package config

import (
	"github.com/mitchellh/go-homedir"
	"path"
)

type HomeDir string

func (m *MarketConfig) HomePath() (HomeDir, error) {
	path, err := homedir.Expand(m.HomeDir)
	if err != nil {
		return "", err
	}
	return HomeDir(path), nil
}

func (m *MarketConfig) ConfigPath() (string, error) {
	return m.HomeJoin("config.toml")
}

func (m *MarketConfig) HomeJoin(sep ...string) (string, error) {
	homeDir, err := homedir.Expand(m.HomeDir)
	if err != nil {
		return "", err
	}
	finalPath := homeDir
	for _, p := range sep {
		finalPath = path.Join(finalPath, p)
	}

	return finalPath, nil
}
