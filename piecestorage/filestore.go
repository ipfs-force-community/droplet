package piecestorage

import (
	"context"
	"fmt"
	"github.com/filecoin-project/venus-market/utils"
	xerrors "github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path"
)

type IPreSignOp interface {
	GetReadUrl(context.Context, string) (string, error)
	GetWriteUrl(ctx context.Context, s2 string) (string, error)
}

type IPieceStorage interface {
	Type() Protocol
	SaveTo(context.Context, string, io.Reader) (int64, error)
	Read(context.Context, string) (io.ReadCloser, error)
	Len(ctx context.Context, string2 string) (int64, error)
	ReadOffset(context.Context, string, int, int) (io.ReadCloser, error)
	Has(context.Context, string) (bool, error)
	Validate(s string) error

	IPreSignOp
}

type fsPieceStorage struct {
	baseUrl string
}

func (f fsPieceStorage) Len(ctx context.Context, s string) (int64, error) {
	st, err := os.Stat(path.Join(f.baseUrl, s))
	if err != nil {
		return 0, err
	}

	if st.IsDir() {
		return 0, xerrors.Errorf("resource %s expect to be a file but found directory", s)
	}
	return st.Size(), err
}

func (f fsPieceStorage) SaveTo(ctx context.Context, s string, r io.Reader) (int64, error) {
	dstPath := path.Join(f.baseUrl, s)
	tempFile, err := ioutil.TempFile("", "piece-*")
	if err != nil {
		return 0, err
	}

	defer tempFile.Close()
	wlen, err := io.Copy(tempFile, r)
	if err != nil {
		return -1, fmt.Errorf("unable to write file to %s %w", dstPath, err)
	}
	err = utils.Move(tempFile.Name(), dstPath)
	return wlen, err
}

func (f fsPieceStorage) Read(ctx context.Context, s string) (io.ReadCloser, error) {
	return os.Open(path.Join(f.baseUrl, s))
}

func (f fsPieceStorage) ReadOffset(ctx context.Context, s string, offset int, size int) (io.ReadCloser, error) {
	dstPath := path.Join(f.baseUrl, s)
	fs, err := os.Open(dstPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s %w", dstPath, err)
	}
	_, err = fs.Seek(int64(offset), 0)
	if err != nil {
		return nil, fmt.Errorf("unable to seek position to %din file %s %w", offset, dstPath, err)
	}
	return utils.NewLimitedBufferReader(fs, int(size)), nil
}

func (f fsPieceStorage) Has(ctx context.Context, s string) (bool, error) {
	_, err := os.Stat(path.Join(f.baseUrl, s))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f fsPieceStorage) Validate(s string) error {
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

func (f fsPieceStorage) Type() Protocol {
	return FS
}

func (f fsPieceStorage) GetReadUrl(ctx context.Context, s2 string) (string, error) {
	return path.Join(f.baseUrl, s2), nil
}

func (f fsPieceStorage) GetWriteUrl(ctx context.Context, s2 string) (string, error) {
	return path.Join(f.baseUrl, s2), nil
}

func newFsPieceStorage(baseUlr string) (IPieceStorage, error) {
	fs := &fsPieceStorage{baseUrl: baseUlr}
	if err := fs.Validate(baseUlr); err != nil {
		return nil, err
	}
	return fs, nil
}
