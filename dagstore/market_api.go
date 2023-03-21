package dagstore

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs-force-community/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/throttle"
	"github.com/filecoin-project/go-padreader"
	gatewayAPIV2 "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	vSharedTypes "github.com/filecoin-project/venus/venus-shared/types"

	marketMetrics "github.com/filecoin-project/venus-market/v2/metrics"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	"github.com/filecoin-project/venus-market/v2/utils"
)

type MarketAPI interface {
	FetchFromPieceStorage(ctx context.Context, pieceCid cid.Cid) (mount.Reader, error)
	GetUnpaddedCARSize(ctx context.Context, pieceCid cid.Cid) (uint64, error)
	IsUnsealed(ctx context.Context, pieceCid cid.Cid) (bool, error)
	Start(ctx context.Context) error
}

type marketAPI struct {
	pieceStorageMgr     *piecestorage.PieceStorageManager
	pieceRepo           repo.StorageDealRepo
	useTransient        bool
	metricsCtx          metrics.MetricsCtx
	gatewayMarketClient gatewayAPIV2.IMarketClient

	throttle throttle.Throttler
}

var _ MarketAPI = (*marketAPI)(nil)

func NewMarketAPI(
	ctx metrics.MetricsCtx,
	repo repo.Repo,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	gatewayMarketClient gatewayAPIV2.IMarketClient,
	useTransient bool,
	concurrency int) MarketAPI {

	return &marketAPI{
		pieceRepo:           repo.StorageDealRepo(),
		pieceStorageMgr:     pieceStorageMgr,
		useTransient:        useTransient,
		metricsCtx:          ctx,
		gatewayMarketClient: gatewayMarketClient,

		throttle: throttle.Fixed(concurrency),
	}
}

func (m *marketAPI) Start(_ context.Context) error {
	return nil
}

func (m *marketAPI) IsUnsealed(ctx context.Context, pieceCid cid.Cid) (bool, error) {
	_, err := m.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		log.Warnf("unable to find storage for piece %s: %s", pieceCid, err)

		// check it from the SP through venus-gateway
		deals, err := m.pieceRepo.GetDealsByPieceCidAndStatus(ctx, pieceCid, storageprovider.ReadyRetrievalDealStatus...)
		if err != nil {
			return false, fmt.Errorf("get delas for piece %s: %w", pieceCid, err)
		}

		if len(deals) == 0 {
			return false, fmt.Errorf("no storage deals found for piece %s", pieceCid)
		}

		// check if we have an unsealed deal for the given piece in any of the unsealed sectors.
		for _, deal := range deals {
			deal := deal

			var isUnsealed bool
			// Throttle this path to avoid flooding the storage subsystem.
			err := m.throttle.Do(ctx, func(ctx context.Context) (err error) {
				// todo ProofType can not be passed, SP processes itself?
				isUnsealed, err = m.gatewayMarketClient.IsUnsealed(ctx, deal.Proposal.Provider, pieceCid,
					deal.SectorNumber,
					vSharedTypes.PaddedByteIndex(deal.Offset.Unpadded()),
					deal.Proposal.PieceSize)
				if err != nil {
					return fmt.Errorf("failed to check if sector %d for deal %d was unsealed: %w", deal.SectorNumber, deal.DealID, err)
				}

				if isUnsealed {
					// send SectorsUnsealPiece task
					wps, err := m.pieceStorageMgr.FindStorageForWrite(int64(deal.Proposal.PieceSize))
					if err != nil {
						return fmt.Errorf("failed to find storage to write %s: %w", pieceCid, err)
					}

					pieceTransfer, err := wps.GetPieceTransfer(ctx, pieceCid.String())
					if err != nil {
						return fmt.Errorf("get piece transfer for %s: %w", pieceCid, err)
					}

					return m.gatewayMarketClient.SectorsUnsealPiece(
						ctx,
						deal.Proposal.Provider,
						pieceCid,
						deal.SectorNumber,
						vSharedTypes.PaddedByteIndex(deal.Offset.Unpadded()),
						deal.Proposal.PieceSize,
						pieceTransfer,
					)
				}

				return nil
			})

			if err != nil {
				log.Warnf("failed to check/retrieve unsealed sector: %s", err)
				continue // move on to the next match.
			}

			if isUnsealed {
				return true, nil
			}
		}

		// we don't have an unsealed sector containing the piece
		return false, nil
	}

	return true, nil
}

func (m *marketAPI) FetchFromPieceStorage(ctx context.Context, pieceCid cid.Cid) (mount.Reader, error) {
	payloadSize, pieceSize, err := m.pieceRepo.GetPieceSize(ctx, pieceCid)
	if err != nil {
		return nil, err
	}

	pieceStorage, err := m.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		return nil, err
	}
	storageName := pieceStorage.GetName()
	size, err := pieceStorage.Len(ctx, pieceCid.String())
	if err != nil {
		return nil, err
	}
	// assume reader always success, wrapper reader for metrics was expensive
	stats.Record(m.metricsCtx, marketMetrics.DagStorePRBytesRequested.M(size))
	_ = stats.RecordWithTags(m.metricsCtx, []tag.Mutator{tag.Upsert(marketMetrics.StorageNameTag, storageName)}, marketMetrics.StorageRetrievalHitCount.M(1))
	if m.useTransient {
		// only need reader stream
		r, err := pieceStorage.GetReaderCloser(ctx, pieceCid.String())
		if err != nil {
			return nil, err
		}

		padR, err := padreader.NewInflator(r, payloadSize, pieceSize.Unpadded())
		if err != nil {
			return nil, err
		}
		stats.Record(m.metricsCtx, marketMetrics.DagStorePRInitCount.M(1))
		return &mountWrapper{r, padR}, nil
	}
	// must support Seek/ReadAt
	r, err := pieceStorage.GetMountReader(ctx, pieceCid.String())
	if err != nil {
		return nil, err
	}
	stats.Record(m.metricsCtx, marketMetrics.DagStorePRInitCount.M(1))
	return utils.NewAlgnZeroMountReader(r, int(payloadSize), int(pieceSize)), nil
}

func (m *marketAPI) GetUnpaddedCARSize(ctx context.Context, pieceCid cid.Cid) (uint64, error) {
	pieceInfo, err := m.pieceRepo.GetPieceInfo(ctx, pieceCid)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pieceInfo for piece %s: %w", pieceCid, err)
	}

	if len(pieceInfo.Deals) == 0 {
		return 0, fmt.Errorf("no storage deals found for piece %s", pieceCid)
	}

	len := pieceInfo.Deals[0].Length

	return uint64(len), nil
}

type mountWrapper struct {
	closeR io.ReadCloser
	readR  io.Reader
}

var _ mount.Reader = (*mountWrapper)(nil)

func (r *mountWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, fmt.Errorf("ReadAt called but not implemented")
}

func (r *mountWrapper) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("Seek called but not implemented")
}

func (r *mountWrapper) Read(p []byte) (n int, err error) {
	n, err = r.readR.Read(p)
	return
}

func (r *mountWrapper) Close() error {
	return r.closeR.Close()
}
