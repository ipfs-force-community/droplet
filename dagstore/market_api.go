package dagstore

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/go-padreader"

	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/filecoin-project/venus-market/utils"
)

type MarketAPI interface {
	FetchUnsealedPiece(ctx context.Context, pieceCid cid.Cid) (mount.Reader, error)
	GetUnpaddedCARSize(ctx context.Context, pieceCid cid.Cid) (uint64, error)
	IsUnsealed(ctx context.Context, pieceCid cid.Cid) (bool, error)
	Start(ctx context.Context) error
}

type marketAPI struct {
	pieceStorageMgr *piecestorage.PieceStorageManager
	pieceRepo       repo.StorageDealRepo
	useTransient    bool
}

var _ MarketAPI = (*marketAPI)(nil)

func NewMarketAPI(repo repo.Repo, pieceStorageMgr *piecestorage.PieceStorageManager, useTransient bool) MarketAPI {
	return &marketAPI{
		pieceRepo:       repo.StorageDealRepo(),
		pieceStorageMgr: pieceStorageMgr,
		useTransient:    useTransient,
	}
}

func (m *marketAPI) Start(_ context.Context) error {
	return nil
}

func (m *marketAPI) IsUnsealed(ctx context.Context, pieceCid cid.Cid) (bool, error) {
	_, err := m.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		return false, fmt.Errorf("unable to find storage for piece %s %w", pieceCid, err)
	}
	return true, nil
	//todo check isunseal from miner
}

func (m *marketAPI) FetchUnsealedPiece(ctx context.Context, pieceCid cid.Cid) (mount.Reader, error) {
	payloadSize, pieceSize, err := m.pieceRepo.GetPieceSize(ctx, pieceCid)
	if err != nil {
		return nil, err
	}

	pieceStorage, err := m.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		return nil, err
	}
	if m.useTransient {
		//only need reader stream
		r, err := pieceStorage.GetReaderCloser(ctx, pieceCid.String())
		if err != nil {
			return nil, err
		}

		padR, err := padreader.NewInflator(r, payloadSize, pieceSize.Unpadded())
		if err != nil {
			return nil, err
		}
		return &mountWrapper{r, padR}, nil
	}
	//must support seek/readeat
	r, err := pieceStorage.GetMountReader(ctx, pieceCid.String())
	if err != nil {
		return nil, err
	}
	return utils.NewAlgnZeroMountReader(r, int(payloadSize), int(pieceSize)), nil
}

func (m *marketAPI) GetUnpaddedCARSize(ctx context.Context, pieceCid cid.Cid) (uint64, error) {
	pieceInfo, err := m.pieceRepo.GetPieceInfo(ctx, pieceCid)
	if err != nil {
		return 0, xerrors.Errorf("failed to fetch pieceInfo for piece %s: %w", pieceCid, err)
	}

	if len(pieceInfo.Deals) == 0 {
		return 0, xerrors.Errorf("no storage deals found for piece %s", pieceCid)
	}

	len := pieceInfo.Deals[0].Length

	// todois this size need to convert to unpad size
	return uint64(len), nil
}

type mountWrapper struct {
	closeR io.ReadCloser
	readR  io.Reader
}

var _ mount.Reader = (*mountWrapper)(nil)

func (r *mountWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, xerrors.Errorf("ReadAt called but not implemented")
}

func (r *mountWrapper) Seek(offset int64, whence int) (int64, error) {
	return 0, xerrors.Errorf("Seek called but not implemented")
}
func (r *mountWrapper) Read(p []byte) (n int, err error) {
	return r.readR.Read(p)
}

func (r *mountWrapper) Close() error {
	return r.closeR.Close()
}
