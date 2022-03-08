package storageprovider

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

type DealAssiger interface {
	MarkDealsAsPacking(ctx context.Context, miner address.Address, dealIDs []abi.DealID) error
	UpdateDealOnPacking(ctx context.Context, miner address.Address, dealID abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error
	UpdateDealStatus(ctx context.Context, miner address.Address, dealID abi.DealID, pieceStatus string) error
	GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*types.DealInfo, error)
	GetUnPackedDeals(ctx context.Context, miner address.Address, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error)
	AssignUnPackedDeals(ctx context.Context, miner address.Address, ssize abi.SectorSize, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error)
}

var _ DealAssiger = (*dealAssigner)(nil)

// NewProviderPieceStore creates a statestore for storing metadata about pieces
// shared by the piecestorage and retrieval providers
func NewDealAssigner(lc fx.Lifecycle, pieceStorage *config.PieceStorage, r repo.Repo) (DealAssiger, error) {
	ps, err := newPieceStoreEx(pieceStorage, r)
	if err != nil {
		return nil, xerrors.Errorf("construct extend piece store %w", err)
	}
	return ps, nil
}

type dealAssigner struct {
	pieceStorage config.PieceStorage
	repo         repo.Repo
}

// NewDsPieceStore returns a new piecestore based on the given datastore
func newPieceStoreEx(pieceStorage *config.PieceStorage, r repo.Repo) (DealAssiger, error) {
	return &dealAssigner{
		pieceStorage: *pieceStorage,

		repo: r,
	}, nil
}

func (ps *dealAssigner) MarkDealsAsPacking(ctx context.Context, miner address.Address, dealIDs []abi.DealID) error {
	for _, dealID := range dealIDs {
		md, err := ps.repo.StorageDealRepo().GetDealByDealID(ctx, miner, dealID)
		if err != nil {
			log.Error("get deal [%d] error for %s", dealID, miner)
			return xerrors.Errorf("failed to get deal %d for miner %s: %w", dealID, miner.String(), err)
		}

		md.PieceStatus = types.Assigned
		if err := ps.repo.StorageDealRepo().SaveDeal(ctx, md); err != nil {
			return xerrors.Errorf("failed to update deal %d piece status for miner %s: %w", dealID, miner.String(), err)
		}
	}

	return nil
}

//
func (ps *dealAssigner) UpdateDealOnPacking(ctx context.Context, miner address.Address, dealID abi.DealID, sectorID abi.SectorNumber, offset abi.PaddedPieceSize) error {
	md, err := ps.repo.StorageDealRepo().GetDealByDealID(ctx, miner, dealID)
	if err != nil {
		log.Error("get deal [%d] error for %s", dealID, miner)
		return xerrors.Errorf("failed to get deal %d for miner %s: %w", dealID, miner.String(), err)
	}

	md.PieceStatus = types.Assigned
	md.Offset = offset
	md.SectorNumber = sectorID
	if err := ps.repo.StorageDealRepo().SaveDeal(ctx, md); err != nil {
		return xerrors.Errorf("failed to update deal %d piece status for miner %s: %w", dealID, miner.String(), err)
	}

	return nil
}

// Store `dealInfo` in the dealAssigner with key `pieceCID`.
func (ps *dealAssigner) UpdateDealStatus(ctx context.Context, miner address.Address, dealID abi.DealID, pieceStatus string) error {
	md, err := ps.repo.StorageDealRepo().GetDealByDealID(ctx, miner, dealID)
	if err != nil {
		log.Error("get deal [%d] error for %s", dealID, miner)
		return xerrors.Errorf("failed to get deal %d for miner %s: %w", dealID, miner.String(), err)
	}

	md.PieceStatus = pieceStatus
	if err := ps.repo.StorageDealRepo().SaveDeal(ctx, md); err != nil {
		return xerrors.Errorf("failed to update deal %d piece status for miner %s: %w", dealID, miner.String(), err)
	}

	return nil
}

func (ps *dealAssigner) GetDeals(ctx context.Context, mAddr address.Address, pageIndex, pageSize int) ([]*types.DealInfo, error) {
	var dis []*types.DealInfo

	mds, err := ps.repo.StorageDealRepo().GetDeals(ctx, mAddr, pageIndex, pageSize)
	if err != nil {
		return nil, err
	}

	for _, md := range mds {
		// TODO: 要排除不可密封状态的订单?
		if md.DealID > 0 && !isTerminateState(md) {
			dis = append(dis, &types.DealInfo{

				DealInfo: piecestore.DealInfo{
					DealID:   md.DealID,
					SectorID: md.SectorNumber,
					Offset:   md.Offset,
					Length:   md.Proposal.PieceSize,
				},
				ClientDealProposal: md.ClientDealProposal,
				TransferType:       md.Ref.TransferType,
				Root:               md.Ref.Root,
				PublishCid:         *md.PublishCid,
				FastRetrieval:      md.FastRetrieval,
				Status:             md.PieceStatus,
			})
		}
	}

	return dis, nil
}

var defaultMaxPiece = 10
var defaultGetDealSpec = &types.GetDealSpec{
	MaxPiece:     defaultMaxPiece,
	MaxPieceSize: 0,
}

func (ps *dealAssigner) GetUnPackedDeals(ctx context.Context, miner address.Address, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error) {
	if spec == nil {
		spec = defaultGetDealSpec
	}
	if spec.MaxPiece == 0 {
		spec.MaxPiece = defaultMaxPiece
	}

	mds, err := ps.repo.StorageDealRepo().GetDealsByPieceStatus(ctx, miner, types.Undefine)
	if err != nil {
		return nil, err
	}

	var (
		result       []*types.DealInfoIncludePath
		numberPiece  int
		curPieceSize uint64
	)

	for _, md := range mds {
		// TODO: 要排除不可密封状态的订单?
		if md.DealID == 0 || isTerminateState(md) {
			continue
		}
		if ((spec.MaxPieceSize > 0 && uint64(md.Proposal.PieceSize)+curPieceSize < spec.MaxPieceSize) || spec.MaxPieceSize == 0) && numberPiece+1 < spec.MaxPiece {
			result = append(result, &types.DealInfoIncludePath{
				DealProposal:    md.Proposal,
				Offset:          md.Offset,
				Length:          md.Proposal.PieceSize,
				PayloadSize:     md.PayloadSize,
				DealID:          md.DealID,
				TotalStorageFee: md.Proposal.TotalStorageFee(),
				FastRetrieval:   md.FastRetrieval,
				PublishCid:      *md.PublishCid,
			})

			curPieceSize += uint64(md.Proposal.PieceSize)
			numberPiece++
		}
	}

	return result, nil
}

func (ps *dealAssigner) AssignUnPackedDeals(ctx context.Context, miner address.Address, ssize abi.SectorSize, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error) {
	deals, err := ps.GetUnPackedDeals(ctx, miner, &types.GetDealSpec{MaxPiece: math.MaxInt32}) //TODO get all pending deals ???
	if err != nil {
		return nil, err
	}

	if len(deals) == 0 {
		return nil, nil
	}

	// 按照尺寸, 时间, 价格排序
	sort.Slice(deals, func(i, j int) bool {
		left, right := deals[i], deals[j]
		if left.PieceSize != right.PieceSize {
			return left.PieceSize < right.PieceSize
		}

		if left.StartEpoch != right.StartEpoch {
			return left.StartEpoch < right.StartEpoch
		}

		return left.StoragePricePerEpoch.GreaterThan(right.StoragePricePerEpoch)
	})

	pieces, err := pickAndAlign(deals, ssize, spec)
	if err != nil {
		return nil, fmt.Errorf("unable to pick and align pieces from deals: %w", err)
	}

	if len(pieces) == 0 {
		return nil, nil
	}

	// TODO: is this concurrent safe?
	if err := ps.repo.Transaction(func(txRepo repo.TxRepo) error {
		for _, piece := range pieces {
			md, err := txRepo.StorageDealRepo().GetDealByDealID(ctx, miner, piece.DealID)
			if err != nil {
				return err
			}

			md.PieceStatus = types.Assigned
			if err := txRepo.StorageDealRepo().SaveDeal(ctx, md); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return pieces, nil
}

type CombinedPieces struct {
	Pieces     []*types.DealInfoIncludePath
	DealIDs    []abi.DealID
	MinStart   abi.ChainEpoch
	PriceTotal abi.TokenAmount
}
