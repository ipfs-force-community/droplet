package piece

import (
	"context"
	"io"
	"path"

	"github.com/filecoin-project/go-state-types/abi"
)

type IPieceStorage interface {
	SaveTo(context.Context, string, io.Reader) (int64, error)
	Read(context.Context, string) (io.ReadCloser, error)
	ReadOffset(context.Context, string, abi.UnpaddedPieceSize, abi.UnpaddedPieceSize) (io.ReadCloser, error)
	Has(string) (bool, error)
}

var _ IPieceStorage = (*PieceStorage)(nil)

type PieceStorage struct {
	path string
}

func (p *PieceStorage) SaveTo(ctx context.Context, s string, reader io.Reader) (int64, error) {
	return ReWrite(path.Join(p.path, s), reader)
}

func (p *PieceStorage) Read(ctx context.Context, s string) (io.ReadCloser, error) {
	return Read(path.Join(p.path, s))
}

func (p *PieceStorage) ReadOffset(ctx context.Context, s string, offset, size abi.UnpaddedPieceSize) (io.ReadCloser, error) {
	return ReadOffset(path.Join(p.path, s), int(offset), int(size))
}

func (p *PieceStorage) Has(s string) (bool, error) {
	return Has(path.Join(p.path, s))
}
