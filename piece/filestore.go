package piece

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"io"
	"os"
	"path"
)

type IPieceStorage interface {
	SaveTo(context.Context, cid.Cid, io.Reader) (int64, error)
	Read(context.Context, cid.Cid) (io.ReadCloser, error)
	ReadSize(context.Context, cid.Cid, abi.UnpaddedPieceSize, abi.UnpaddedPieceSize) (io.ReadCloser, error)
	Has(cid.Cid) (bool, error)
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

func (p *PieceFileStorage) SaveTo(ctx context.Context, s cid.Cid, reader io.Reader) (int64, error) {
	fs, err := os.OpenFile(path.Join(p.path, s.String()), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return 0, err
	}
	defer fs.Close()
	return io.Copy(fs, reader)
}

func (p *PieceFileStorage) Read(ctx context.Context, s cid.Cid) (io.ReadCloser, error) {
	return os.Open(path.Join(p.path, s.String()))
}

func (p *PieceFileStorage) ReadSize(ctx context.Context, s cid.Cid, offset, size abi.UnpaddedPieceSize) (io.ReadCloser, error) {
	fs, err := os.Open(path.Join(p.path, s.String()))
	if err != nil {
		return nil, err
	}
	_, err = fs.Seek(int64(offset), 0)
	if err != nil {
		return nil, err
	}
	return NewLimitedBufferReader(fs, int(size)), nil
}

func (p *PieceFileStorage) Has(s cid.Cid) (bool, error) {
	_, err := os.Stat(path.Join(p.path, s.String()))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil

}
