package piece

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	"golang.org/x/xerrors"
	"io"
	"os"
	"path"
)

type IPieceStorage interface {
	SaveTo(context.Context, string, io.Reader) (int64, error)
	Read(context.Context, string) (io.ReadCloser, error)
	ReadOffset(context.Context, string, abi.UnpaddedPieceSize, abi.UnpaddedPieceSize) (io.ReadCloser, error)
	Has(string) (bool, error)
}

var _ IPieceStorage = (*PieceFileStorage)(nil)

type PieceFileStorage struct {
	path string
}

func NewPieceFileStorage(piecePath string) (*PieceFileStorage, error) {
	st, err := os.Stat(piecePath)
	if err != nil {
		return nil, err
	}

	if !st.IsDir() {
		return nil, xerrors.Errorf("expect a directory but got file")
	}
	return &PieceFileStorage{piecePath}, nil
}

func (p *PieceFileStorage) SaveTo(ctx context.Context, s string, reader io.Reader) (int64, error) {
	fs, err := os.Create(path.Join(p.path, s))
	if err != nil {
		return 0, err
	}

	defer func() {
		_ = fs.Close()
	}()
	return io.Copy(fs, reader)
}

func (p *PieceFileStorage) Read(ctx context.Context, s string) (io.ReadCloser, error) {
	return os.Open(path.Join(p.path, s))
}

func (p *PieceFileStorage) ReadOffset(ctx context.Context, s string, offset, size abi.UnpaddedPieceSize) (io.ReadCloser, error) {
	fs, err := os.Open(path.Join(p.path, s))
	if err != nil {
		return nil, err
	}
	_, err = fs.Seek(int64(offset), 0)
	if err != nil {
		return nil, err
	}
	return NewLimitedBufferReader(fs, int(size)), nil
}

func (p *PieceFileStorage) Has(s string) (bool, error) {
	_, err := os.Stat(path.Join(p.path, s))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
