package config

import (
	"github.com/mitchellh/go-homedir"
	"path"
)

type HomeDir string

func (m *Market) HomePath() (string, error) {
	return homedir.Expand(m.HomeDir)
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
