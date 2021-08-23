package config

import (
	"github.com/mitchellh/go-homedir"
	"path"
)

type HomeDir string

func (m *Market) HomePath() (HomeDir, error) {
	path, err := homedir.Expand(m.HomeDir)
	if err != nil {
		return "", err
	}
	return HomeDir(path), nil
}

func (m *Market) HomeJoin(sep ...string) (string, error) {
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
