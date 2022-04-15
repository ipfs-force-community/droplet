package rpc

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/xerrors"
)

const (
	DefDealsDir = "/tmp/deals"
	DealsPrefix = "deal"
)

func DealServer(mux *mux.Router, dealsDir string) error {
	if len(dealsDir) == 0 {
		dealsDir = DefDealsDir
	}
	dpath := "/" + DealsPrefix + "/"
	if err := os.MkdirAll(dealsDir, 0755); err != nil {
		return fmt.Errorf("failed to mk directory %s for deals: %w", dealsDir, err)
	}
	fileSystem := &FileOpener{Dir: dealsDir}
	mux.PathPrefix(dpath).Handler(http.StripPrefix(dpath, http.FileServer(fileSystem)))

	return nil
}

type FileOpener struct {
	Dir string
}

func (fo *FileOpener) Open(name string) (http.File, error) {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, xerrors.New("http: invalid character in file path")
	}
	dir := fo.Dir
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	f, err := os.Open(fullName)
	if err != nil {
		return nil, mapDirOpenError(err, fullName)
	}
	return f, nil
}

func mapDirOpenError(originalErr error, name string) error {
	if os.IsNotExist(originalErr) || os.IsPermission(originalErr) {
		return originalErr
	}

	parts := strings.Split(name, string(filepath.Separator))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		fi, err := os.Stat(strings.Join(parts[:i+1], string(filepath.Separator)))
		if err != nil {
			return originalErr
		}
		if !fi.IsDir() {
			return fs.ErrNotExist
		}
	}
	return originalErr
}
