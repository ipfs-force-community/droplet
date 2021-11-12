package piece

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"math"
	"math/bits"
	"path"
	"sort"

	"github.com/filecoin-project/go-commp-utils/zerocomm"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	market2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/market"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
)

var log = logging.Logger("piece")

type DealAssiger interface {
	MarkDealsAsPacking(ctx context.Context, miner address.Address, dealIDs []abi.DealID) error
	UpdateDealOnPacking(ctx context.Context, miner address.Address, dealID abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error
	UpdateDealStatus(ctx context.Context, miner address.Address, dealID abi.DealID, pieceStatus string) error
	GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*DealInfo, error)
	GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error)
	ListPieceInfoKeys() ([]cid.Cid, error)
	GetUnPackedDeals(ctx context.Context, miner address.Address, spec *GetDealSpec) ([]*DealInfoIncludePath, error)
	AssignUnPackedDeals(ctx context.Context, miner address.Address, ssize abi.SectorSize, spec *GetDealSpec) ([]*DealInfoIncludePath, error)
}

var _ DealAssiger = (*dealAssigner)(nil)

// NewProviderPieceStore creates a statestore for storing metadata about pieces
// shared by the piecestorage and retrieval providers
func NewDealAssigner(lc fx.Lifecycle, pieceStorage *config.PieceStorageString, r repo.Repo) (DealAssiger, error) {
	ps, err := newPieceStoreEx(pieceStorage, r.StorageDealRepo())
	if err != nil {
		return nil, xerrors.Errorf("construct extend piece store %w", err)
	}
	return ps, nil
}

type dealAssigner struct {
	pieceStorage    config.PieceStorageString
	StorageDealRepo repo.StorageDealRepo
}

// NewDsPieceStore returns a new piecestore based on the given datastore
func newPieceStoreEx(pieceStorage *config.PieceStorageString, storageDealRepo repo.StorageDealRepo) (DealAssiger, error) {
	return &dealAssigner{
		pieceStorage: *pieceStorage,

		StorageDealRepo: storageDealRepo,
	}, nil
}

func (ps *dealAssigner) Start(ctx context.Context) error {
	return nil
}

func (ps *dealAssigner) OnReady(ready shared.ReadyFunc) {
	ready(nil)
}

// Store `dealInfo` in the dealAssigner with key `pieceCID`.
// piece的存取改为从StorageDealRepo获取
func (ps *dealAssigner) AddDealForPiece(pieceCID cid.Cid, dealInfo piecestore.DealInfo) error {

	return nil
}

// Retrieve the PieceInfo associated with `pieceCID` from the piece info store.
func (ps *dealAssigner) GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error) {
	pi, err := ps.StorageDealRepo.GetPieceInfo(pieceCID)
	if err != nil {
		return piecestore.PieceInfo{}, err
	}

	return *pi, err
}

func (ps *dealAssigner) ListPieceInfoKeys() ([]cid.Cid, error) {
	return ps.StorageDealRepo.ListPieceInfoKeys()
}

func (ps *dealAssigner) MarkDealsAsPacking(ctx context.Context, miner address.Address, dealIDs []abi.DealID) error {
	for _, dealID := range dealIDs {
		md, err := ps.StorageDealRepo.GetDealByDealID(miner, dealID)
		if err != nil {
			log.Error("get deal [%d] error for %s", dealID, miner)
			return xerrors.Errorf("failed to get deal %d for miner %s: %w", dealID, miner.String(), err)
		}

		md.PieceStatus = Assigned
		if err := ps.StorageDealRepo.SaveDeal(md); err != nil {
			return xerrors.Errorf("failed to update deal %d piece status for miner %s: %w", dealID, miner.String(), err)
		}
	}

	return nil
}

//
func (ps *dealAssigner) UpdateDealOnPacking(ctx context.Context, miner address.Address, dealID abi.DealID, sectorID abi.SectorNumber, offset abi.PaddedPieceSize) error {
	md, err := ps.StorageDealRepo.GetDealByDealID(miner, dealID)
	if err != nil {
		log.Error("get deal [%d] error for %s", dealID, miner)
		return xerrors.Errorf("failed to get deal %d for miner %s: %w", dealID, miner.String(), err)
	}

	md.PieceStatus = Assigned
	md.Offset = offset
	md.SectorNumber = sectorID
	if err := ps.StorageDealRepo.SaveDeal(md); err != nil {
		return xerrors.Errorf("failed to update deal %d piece status for miner %s: %w", dealID, miner.String(), err)
	}

	return nil
}

// Store `dealInfo` in the dealAssigner with key `pieceCID`.
func (ps *dealAssigner) UpdateDealStatus(ctx context.Context, miner address.Address, dealID abi.DealID, pieceStatus string) error {
	md, err := ps.StorageDealRepo.GetDealByDealID(miner, dealID)
	if err != nil {
		log.Error("get deal [%d] error for %s", dealID, miner)
		return xerrors.Errorf("failed to get deal %d for miner %s: %w", dealID, miner.String(), err)
	}

	md.PieceStatus = pieceStatus
	if err := ps.StorageDealRepo.SaveDeal(md); err != nil {
		return xerrors.Errorf("failed to update deal %d piece status for miner %s: %w", dealID, miner.String(), err)
	}

	return nil
}

func (ps *dealAssigner) GetDeals(ctx context.Context, mAddr address.Address, pageIndex, pageSize int) ([]*DealInfo, error) {
	var dis []*DealInfo

	mds, err := ps.StorageDealRepo.GetDeals(mAddr, pageIndex, pageSize)
	if err != nil {
		return nil, err
	}

	for _, md := range mds {
		dis = append(dis, &DealInfo{
			DealInfo: piecestore.DealInfo{
				DealID:   md.DealID,
				SectorID: md.SectorNumber,
				Offset:   md.Offset,
				Length:   md.Proposal.PieceSize,
			},
			TransferType:  md.Ref.TransferType,
			Root:          md.Ref.Root,
			PublishCid:    *md.PublishCid,
			FastRetrieval: md.FastRetrieval,
			Status:        md.PieceStatus,
		})
	}

	return dis, nil
}

var defaultMaxPiece = 10
var defaultGetDealSpec = &GetDealSpec{
	MaxPiece:     defaultMaxPiece,
	MaxPieceSize: 0,
}

func (ps *dealAssigner) GetUnPackedDeals(ctx context.Context, miner address.Address, spec *GetDealSpec) ([]*DealInfoIncludePath, error) {
	if spec == nil {
		spec = defaultGetDealSpec
	}
	if spec.MaxPiece == 0 {
		spec.MaxPiece = defaultMaxPiece
	}

	mds, err := ps.StorageDealRepo.GetDealsByPieceStatus(miner, Undefine)
	if err != nil {
		return nil, err
	}

	var (
		result       []*DealInfoIncludePath
		numberPiece  int
		curPieceSize uint64
	)
	for _, md := range mds {
		if uint64(md.Length)+curPieceSize < spec.MaxPieceSize && numberPiece+1 < spec.MaxPiece {
			result = append(result, &DealInfoIncludePath{
				DealProposal:    md.Proposal,
				Offset:          md.Offset,
				Length:          md.Length,
				DealID:          md.DealID,
				TotalStorageFee: md.Proposal.TotalStorageFee(),
				PieceStorage:    path.Join(string(ps.pieceStorage), md.Proposal.PieceCID.String()),
				FastRetrieval:   md.FastRetrieval,
				PublishCid:      *md.PublishCid,
			})
			md.PieceStatus = Assigned
			if err := ps.StorageDealRepo.SaveDeal(md); err != nil {
				return nil, err
			}

			curPieceSize += uint64(md.Length)
			numberPiece++
		}
	}

	return result, nil
}

func (ps *dealAssigner) AssignUnPackedDeals(ctx context.Context, miner address.Address, ssize abi.SectorSize, spec *GetDealSpec) ([]*DealInfoIncludePath, error) {
	deals, err := ps.GetUnPackedDeals(ctx, miner, &GetDealSpec{MaxPiece: math.MaxInt32}) //TODO get all pending deals ???
	if err != nil {
		return nil, err
	}

	if len(deals) == 0 {
		return nil, nil
	}

	// 按照尺寸, 时间, 价格排序
	sort.Slice(deals, func(i, j int) bool {
		left, right := deals[i], deals[j]
		if left.PieceSize.Unpadded() != right.PieceSize.Unpadded() {
			return left.PieceSize.Unpadded() < right.PieceSize.Unpadded()
		}

		if left.StartEpoch != right.StartEpoch {
			return left.StartEpoch < right.StartEpoch
		}

		return left.StoragePricePerEpoch.GreaterThan(right.StoragePricePerEpoch)
	})

	dealsBySize := [][]*DealInfoIncludePath{}
	dealSizeIdxMap := map[abi.UnpaddedPieceSize]int{}

	// 按尺寸分组
	sectorCap := abi.PaddedPieceSize(ssize).Unpadded()
	for di, deal := range deals {
		if deal.PieceSize.Unpadded() > sectorCap {
			log.Infow("deals too large are ignored", "count", len(deals[di:]), "gt", deal.PieceSize.Unpadded(), "max", sectorCap)
			break
		}

		length := len(dealsBySize)
		if length == 0 {
			dealsBySize = append(dealsBySize, []*DealInfoIncludePath{deal})
			dealSizeIdxMap[deal.PieceSize.Unpadded()] = length
			continue
		}

		last := length - 1

		if deal.PieceSize.Unpadded() != dealsBySize[last][0].PieceSize.Unpadded() {
			dealsBySize = append(dealsBySize, []*DealInfoIncludePath{deal})
			dealSizeIdxMap[deal.PieceSize.Unpadded()] = length
			continue
		}

		dealsBySize[last] = append(dealsBySize[last], deal)
	}

	// 合并
	fillers, err := fillersFromRem(sectorCap)
	if err != nil {
		log.Warnw("unable to get fillers", "size", sectorCap, "err", err)
		return nil, err
	}
	combinedAll := make([]*CombinedPieces, 0, len(deals))
	for i := range dealsBySize {
		if len(dealsBySize[i]) == 0 {
			continue
		}

		// 消费掉当前尺寸内的所有订单
		for len(dealsBySize[i]) > 0 {
			first := dealsBySize[i][0]
			dealsBySize[i] = dealsBySize[i][1:]

			dlog := log.With("first", first.DealID, "first-size", first.PieceSize.Unpadded())

			dlog.Info("init combined deals")
			combined := &CombinedPieces{
				Pieces:     []*DealInfoIncludePath{first},
				DealIDs:    []abi.DealID{first.DealID},
				MinStart:   first.StartEpoch,
				PriceTotal: first.TotalStorageFee,
			}

			// 遍历所有填充尺寸
			for i, fsize := range fillers {
				var dealOfFsize *DealInfoIncludePath

				// 如果允许填充更多订单, 尝试找出当前填充尺寸对应的下一个订单
				if len(combined.DealIDs) < spec.MaxPiece {
					if sizeIdx, has := dealSizeIdxMap[fsize]; has && len(dealsBySize[sizeIdx]) > 0 {
						dealOfFsize = dealsBySize[sizeIdx][0]
						dealsBySize[sizeIdx] = dealsBySize[sizeIdx][1:]
					}
				}

				// 填充 全0 piece
				if dealOfFsize == nil {
					combined.Pieces = append(combined.Pieces, &DealInfoIncludePath{
						DealProposal: market2.DealProposal{
							PieceSize: fsize.Padded(),
							PieceCID:  zerocomm.ZeroPieceCommitment(fsize),
						},
					})
					continue
				}

				dlog.Infow("filling combined deals", "piece", dealOfFsize.DealID, "piece-size", dealOfFsize.PieceSize, "piece-index", i+1)
				// 填充订单 piece
				combined.Pieces = append(combined.Pieces, dealOfFsize)
				combined.DealIDs = append(combined.DealIDs, dealOfFsize.DealID)
				if dealOfFsize.StartEpoch < combined.MinStart {
					combined.MinStart = dealOfFsize.StartEpoch
				}
				combined.PriceTotal = big.Add(combined.PriceTotal, dealOfFsize.TotalStorageFee)

			}

			combinedAll = append(combinedAll, combined)
		}
	}

	// 按开始时间, 价格排序
	sort.Slice(combinedAll, func(i, j int) bool {
		if combinedAll[i].MinStart != combinedAll[j].MinStart {
			return combinedAll[i].MinStart < combinedAll[j].MinStart
		}

		return combinedAll[i].PriceTotal.GreaterThan(combinedAll[j].PriceTotal)
	})

	pieces := []*DealInfoIncludePath{}
	for _, cp := range combinedAll {
		pieces = append(pieces, cp.Pieces...)

	}
	// not atomic opration for deal
	for _, piece := range pieces {
		md, err := ps.StorageDealRepo.GetDealByDealID(miner, piece.DealID)
		if err != nil {
			return nil, err
		}

		md.PieceStatus = Assigned
		if err := ps.StorageDealRepo.SaveDeal(md); err != nil {
			return nil, err
		}
	}
	return pieces, nil
}

func fillersFromRem(in abi.UnpaddedPieceSize) ([]abi.UnpaddedPieceSize, error) {
	// Convert to in-sector bytes for easier math:
	//
	// Sector size to user bytes ratio is constant, e.g. for 1024B we have 1016B
	// of user-usable data.
	//
	// (1024/1016 = 128/127)
	//
	// Given that we can get sector size by simply adding 1/127 of the user
	// bytes
	//
	// (we convert to sector bytes as they are nice round binary numbers)

	toFill := uint64(in + (in / 127))

	// We need to fill the sector with pieces that are powers of 2. Conveniently
	// computers store numbers in binary, which means we can look at 1s to get
	// all the piece sizes we need to fill the sector. It also means that number
	// of pieces is the number of 1s in the number of remaining bytes to fill
	out := make([]abi.UnpaddedPieceSize, bits.OnesCount64(toFill))
	for i := range out {
		// Extract the next lowest non-zero bit
		next := bits.TrailingZeros64(toFill)
		psize := uint64(1) << uint64(next)
		// e.g: if the number is 0b010100, psize will be 0b000100

		// set that bit to 0 by XORing it, so the next iteration looks at the
		// next bit
		toFill ^= psize

		// Add the piece size to the list of pieces we need to create
		out[i] = abi.PaddedPieceSize(psize).Unpadded()
	}
	return out, nil
}

type CombinedPieces struct {
	Pieces     []*DealInfoIncludePath
	DealIDs    []abi.DealID
	MinStart   abi.ChainEpoch
	PriceTotal abi.TokenAmount
}
