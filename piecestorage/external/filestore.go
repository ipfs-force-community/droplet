package external

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/utils"
)

var log = logging.Logger("external-pieces")

type IExternalPieceStorage interface {
	Read(context.Context, string) (io.ReadCloser, error)
	Len(ctx context.Context, string2 string) (int64, error)
	ReadOffset(context.Context, string, int, int) (io.ReadCloser, error)
	Has(context.Context, string) (bool, error)
}

type externalFsPieceStorage struct {
	paths []string
}

func NewExternalFsPieceStorage(cfg *config.ExternalFsPieceStore) (IExternalPieceStorage, error) {
	fs := externalFsPieceStorage{paths: make([]string, 0)}
	for _, psPath := range cfg.Paths {
		if err := fs.validate(psPath); err != nil {
			log.Errorf("%s validate failed: %s", psPath, err.Error())
		} else {
			fs.paths = append(fs.paths, psPath)
		}
	}

	return &fs, nil
}

func (ef externalFsPieceStorage) Len(ctx context.Context, pCidStr string) (int64, error) {
	for _, psPath := range ef.paths {
		st, err := os.Stat(path.Join(psPath, pCidStr))
		if err != nil {
			log.Errorf("%s stat error: %s", pCidStr, err.Error())
			continue
		}

		if st.IsDir() {
			log.Errorf("%s is not a file", pCidStr)
			continue
		}

		return st.Size(), nil
	}

	return 0, fmt.Errorf("file does not exist")
}

func (ef externalFsPieceStorage) Read(ctx context.Context, pCidStr string) (io.ReadCloser, error) {
	for _, psPath := range ef.paths {
		st, err := os.Stat(path.Join(psPath, pCidStr))
		if err != nil {
			log.Errorf("%s stat error: %s", pCidStr, err.Error())
			continue
		}

		if st.IsDir() {
			log.Errorf("%s is not a file", pCidStr)
			continue
		}

		return os.Open(path.Join(psPath, pCidStr))
	}

	return nil, fmt.Errorf("file does not exist")
}

func (ef externalFsPieceStorage) ReadOffset(ctx context.Context, pCidStr string, offset int, size int) (io.ReadCloser, error) {
	for _, psPath := range ef.paths {
		dstPath := path.Join(psPath, pCidStr)
		fs, err := os.Open(dstPath)
		if err != nil {
			log.Errorf("failed to open file %s: %s", pCidStr, err.Error())
			continue
		}

		_, err = fs.Seek(int64(offset), 0)
		if err != nil {
			return nil, fmt.Errorf("failed to seek position to %d in file %s %s", offset, dstPath, err)
		}

		return utils.NewLimitedBufferReader(fs, int(size)), nil
	}

	return nil, fmt.Errorf("file does not exist")
}

func (ef externalFsPieceStorage) Has(ctx context.Context, pCidStr string) (bool, error) {
	for _, psPath := range ef.paths {
		st, err := os.Stat(path.Join(psPath, pCidStr))
		if err != nil {
			log.Errorf("%s stat error: %s", pCidStr, err.Error())
			continue
		}

		if st.IsDir() {
			log.Errorf("%s is not a file", pCidStr)
			continue
		}

		return true, nil
	}

	return false, nil
}

func (ef externalFsPieceStorage) validate(psPath string) error {
	st, err := os.Stat(psPath)
	if err != nil {
		return err
	}

	if !st.IsDir() {
		return xerrors.Errorf("expect a directory but got file")
	}
	return nil
}
