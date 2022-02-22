package config

import (
	"path"

	"github.com/mitchellh/go-homedir"
)

type HomeDir string

type IHome interface {
	HomePath() (HomeDir, error)
	MustHomePath() string
	ConfigPath() (string, error)
	HomeJoin(sep ...string) (string, error)
}
type Home struct {
	HomeDir string `toml:"-"`
}

func (m *Home) HomePath() (HomeDir, error) {
	path, err := homedir.Expand(m.HomeDir)
	if err != nil {
		return "", err
	}
	return HomeDir(path), nil
}

func (m *Home) MustHomePath() string {
	path, _ := homedir.Expand(m.HomeDir)
	return path
}

func (m *Home) ConfigPath() (string, error) {
	return m.HomeJoin("config.toml")
}

func (m *Home) HomeJoin(sep ...string) (string, error) {
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
