package piecestorage

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/pkg/util/fsutil"
)

type fsPieceStorage struct {
	baseUrl string
	fsCfg   *config.FsPieceStorage
}

func (f *fsPieceStorage) Len(ctx context.Context, s string) (int64, error) {
	st, err := os.Stat(path.Join(f.baseUrl, s))
	if err != nil {
		return 0, err
	}

	if st.IsDir() {
		return 0, xerrors.Errorf("resource %s expect to be a file but found directory", s)
	}
	return st.Size(), err
}

func (f *fsPieceStorage) SaveTo(ctx context.Context, s string, r io.Reader) (int64, error) {
	if f.fsCfg.ReadOnly {
		return 0, fmt.Errorf("do not write to a 'readonly' piece store")
	}

	dstPath := path.Join(f.baseUrl, s)
	tempFile, err := ioutil.TempFile("", "piece-*")
	if err != nil {
		return 0, err
	}

	defer func() { _ = tempFile.Close() }()
	wlen, err := io.Copy(tempFile, r)
	if err != nil {
		return -1, fmt.Errorf("unable to write file to %s %w", dstPath, err)
	}
	err = utils.Move(tempFile.Name(), dstPath)
	return wlen, err
}

func (f *fsPieceStorage) GetReaderCloser(ctx context.Context, s string) (io.ReadCloser, error) {
	dstPath := path.Join(f.baseUrl, s)
	fs, err := os.Open(dstPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s %w", dstPath, err)
	}
	return fs, nil
}

func (f *fsPieceStorage) GetMountReader(ctx context.Context, s string) (mount.Reader, error) {
	dstPath := path.Join(f.baseUrl, s)
	fs, err := os.Open(dstPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s %w", dstPath, err)
	}
	return fs, nil
}

func (f *fsPieceStorage) Has(ctx context.Context, s string) (bool, error) {
	_, err := os.Stat(path.Join(f.baseUrl, s))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *fsPieceStorage) Validate(s string) error {
	st, err := os.Stat(f.baseUrl)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(f.baseUrl, 0755)
		}
		return err
	}

	if !st.IsDir() {
		return xerrors.Errorf("expect a directory but got file")
	}
	return nil
}

func (f *fsPieceStorage) CanAllocate(size int64) bool {
	st, err := fsutil.Statfs(f.baseUrl)
	if err != nil {
		log.Warn("unable to get status of %s", f.baseUrl)
		return false
	}
	return st.Available > size
}

func (f *fsPieceStorage) Type() Protocol {
	return FS
}

func (f *fsPieceStorage) ReadOnly() bool {
	return f.fsCfg.ReadOnly
}

func NewFsPieceStorage(fsCfg *config.FsPieceStorage) (IPieceStorage, error) {
	fs := &fsPieceStorage{baseUrl: fsCfg.Path, fsCfg: fsCfg}
	if err := fs.Validate(fsCfg.Path); err != nil {
		return nil, err
	}
	return fs, nil
}
