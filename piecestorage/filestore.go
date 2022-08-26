package piecestorage

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/utils"
	"github.com/filecoin-project/venus/pkg/util/fsutil"
)

type fsPieceStorage struct {
	baseUrl string
	fsCfg   *config.FsPieceStorage
}

func (f *fsPieceStorage) Len(ctx context.Context, resourceId string) (int64, error) {
	st, err := os.Stat(path.Join(f.baseUrl, resourceId))
	if err != nil {
		return 0, err
	}

	if st.IsDir() {
		return 0, fmt.Errorf("resource %s expect to be a file but found directory", resourceId)
	}
	return st.Size(), err
}

func (f *fsPieceStorage) ListResourceIds(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(f.baseUrl)
	if err != nil {
		return nil, err
	}
	var resources []string
	for _, entry := range entries {
		if !entry.IsDir() {
			resources = append(resources, entry.Name())
		}
	}
	return resources, nil
}

func (f *fsPieceStorage) SaveTo(ctx context.Context, resourceId string, r io.Reader) (int64, error) {
	if f.fsCfg.ReadOnly {
		return 0, fmt.Errorf("do not write to a 'readonly' piece store")
	}

	dstPath := path.Join(f.baseUrl, resourceId)
	tempFile, err := ioutil.TempFile("", "piece-*")
	if err != nil {
		return 0, err
	}

	defer func() { _ = tempFile.Close() }()
	wlen, err := io.Copy(tempFile, r)
	if err != nil {
		return -1, fmt.Errorf("unable to write file to %s  %w", dstPath, err)
	}
	err = utils.Move(tempFile.Name(), dstPath)
	return wlen, err
}

func (f *fsPieceStorage) GetReaderCloser(ctx context.Context, resourceId string) (io.ReadCloser, error) {
	dstPath := path.Join(f.baseUrl, resourceId)
	fs, err := os.Open(dstPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s %w", dstPath, err)
	}
	return fs, nil
}

func (f *fsPieceStorage) GetMountReader(ctx context.Context, resourceId string) (mount.Reader, error) {
	dstPath := path.Join(f.baseUrl, resourceId)
	fs, err := os.Open(dstPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s %w", dstPath, err)
	}
	return fs, nil
}

func (f *fsPieceStorage) GetRedirectUrl(_ context.Context, _ string) (string, error) {
	return "", ErrUnsupportRedirect
}

func (f *fsPieceStorage) Has(ctx context.Context, resourceId string) (bool, error) {
	_, err := os.Stat(path.Join(f.baseUrl, resourceId))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *fsPieceStorage) Validate(resourceId string) error {
	st, err := os.Stat(f.baseUrl)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(f.baseUrl, 0755)
		}
		return err
	}

	if !st.IsDir() {
		return fmt.Errorf("expect a directory but got file")
	}
	return nil
}

func (f *fsPieceStorage) GetStorageStatus() (market.StorageStatus, error) {
	st, err := fsutil.Statfs(f.baseUrl)
	if err != nil {
		log.Warn("unable to get status of %s", f.baseUrl)
		return market.StorageStatus{}, nil
	}
	return market.StorageStatus{
		Capacity:  st.Capacity,
		Available: st.Available,
	}, nil
}

func (f *fsPieceStorage) Type() Protocol {
	return FS
}

func (f *fsPieceStorage) ReadOnly() bool {
	return f.fsCfg.ReadOnly
}

func (f *fsPieceStorage) GetName() string {
	return f.fsCfg.Name
}

func NewFsPieceStorage(fsCfg *config.FsPieceStorage) (IPieceStorage, error) {
	fs := &fsPieceStorage{baseUrl: fsCfg.Path, fsCfg: fsCfg}
	if err := fs.Validate(fsCfg.Path); err != nil {
		return nil, err
	}
	return fs, nil
}
