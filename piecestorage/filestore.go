package piecestorage

import (
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"io"
	"os"
	"path"
)

type IPieceStorage interface {
	SaveTo(cid.Cid, io.Reader) (int64, error)
	Read(cid.Cid) (io.ReadCloser, error)
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

func (p *PieceFileStorage) SaveTo(s cid.Cid, reader io.Reader) (int64, error) {
	fs, err := os.OpenFile(path.Join(p.path, s.String()), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return 0, err
	}
	defer fs.Close()
	return io.Copy(fs, reader)
}

func (p *PieceFileStorage) Read(s cid.Cid) (io.ReadCloser, error) {
	return os.Open(path.Join(p.path, s.String()))
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
