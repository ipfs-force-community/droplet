package piece

import (
	"context"
	"encoding/json"
	"github.com/filecoin-project/go-commp-utils/zerocomm"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	market2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus-market/types"
	logging "github.com/ipfs/go-log/v2"
	"math"
	"math/bits"
	"path"
	"sort"
	"strings"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"golang.org/x/xerrors"
	"sync"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared"
)

var log = logging.Logger("piece")

type PieceInfo struct {
	PieceCID cid.Cid
	Deals    []*DealInfo
}

const (
	Undefine = "Undefine"
	Assigned = "Assigned"
	Packing  = "Packing"
	Proving  = "Proving"
)

type DealInfo struct {
	piecestore.DealInfo
	market.ClientDealProposal
	TransferType  string
	Root          cid.Cid
	PublishCid    cid.Cid
	FastRetrieval bool
	Status        string
}

type DealInfoIncludePath struct {
	Offset          abi.PaddedPieceSize
	Length          abi.PaddedPieceSize
	DealID          abi.DealID
	TotalStorageFee abi.TokenAmount
	PieceStorage    string
	market2.DealProposal
	FastRetrieval bool
	PublishCid    cid.Cid
}

type GetDealSpec struct {
	MaxPiece     int
	MaxPieceSize uint64
}

type PieceStore interface {
	UpdateDealOnComplete(pieceCID cid.Cid, proposal market.ClientDealProposal, dataRef *storagemarket.DataRef, publishCid cid.Cid, dealId abi.DealID, fastRetrieval bool) error
	UpdateDealOnPacking(pieceCID cid.Cid, dealId abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error
	UpdateDealStatus(dealId abi.DealID, status string) error
	GetDealByPosition(ctx context.Context, sid abi.SectorID, offset abi.PaddedPieceSize, length abi.PaddedPieceSize) (*DealInfo, error)
	GetDeals(pageIndex, pageSize int) ([]*DealInfo, error)
	AssignUnPackedDeals(spec *GetDealSpec) ([]*DealInfoIncludePath, error)
	GetUnPackedDeals(spec *GetDealSpec) ([]*DealInfoIncludePath, error)
	MarkDealsAsPacking(deals []abi.DealID) error
	ListPieceInfoKeys() ([]cid.Cid, error)
	GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error)

	//jsut mock
	Start(ctx context.Context) error
	OnReady(ready shared.ReadyFunc)
	AddDealForPiece(pieceCID cid.Cid, dealInfo piecestore.DealInfo) error
}

var _ PieceStore = (*dsPieceStore)(nil)

type ExtendPieceStore interface {
	PieceStore
	CIDStore
}

var _ piecestore.PieceStore = (ExtendPieceStore)(nil)

type dsPieceStore struct {
	pieces       datastore.Batching
	pieceStorage *config.PieceStorageString
	pieceLk      sync.Mutex
	ssize        types.SectorSize
}

// NewDsPieceStore returns a new piecestore based on the given datastore
func NewDsPieceStore(ds models.PieceInfoDS, ssize types.SectorSize, pieceStorage *config.PieceStorageString) (PieceStore, error) {
	return &dsPieceStore{
		pieces:       ds,
		pieceStorage: pieceStorage,
		ssize:        ssize,
		pieceLk:      sync.Mutex{},
	}, nil
}

func (ps *dsPieceStore) Start(ctx context.Context) error {
	return nil
}

func (ps *dsPieceStore) OnReady(ready shared.ReadyFunc) {
	ready(nil)
}

// Store `dealInfo` in the PieceStore with key `pieceCID`.
// expire this func just mock here
func (ps *dsPieceStore) AddDealForPiece(pieceCID cid.Cid, dealInfo piecestore.DealInfo) error {
	/*	return ps.mutatePieceInfo(pieceCID, func(pi *PieceInfo) error {
		for _, di := range pi.Deals {
			if di.DealID == dealInfo.DealID {
				return nil
			}
		}
		//new deal
		pi.Deals = append(pi.Deals, DealInfo{
			DealInfo:   dealInfo,
			IsPacking:  false,
			Expiration: 0,
		})
		return nil
	})*/
	return nil
}

func (ps *dsPieceStore) UpdateDealOnComplete(pieceCID cid.Cid, proposal market.ClientDealProposal, dataRef *storagemarket.DataRef, publishCid cid.Cid, dealId abi.DealID, fastRetrieval bool) error {
	return ps.mutatePieceInfo(pieceCID, func(pi *PieceInfo) error {
		for _, di := range pi.Deals {
			if di.DealID == dealId {
				return nil
			}
		}
		//new deal
		pi.Deals = append(pi.Deals, &DealInfo{
			DealInfo: piecestore.DealInfo{
				DealID:   dealId,
				SectorID: 0,
				Offset:   0,
				Length:   proposal.Proposal.PieceSize,
			},
			ClientDealProposal: proposal,
			TransferType:       dataRef.TransferType,
			Root:               dataRef.Root,
			PublishCid:         publishCid,
			FastRetrieval:      fastRetrieval,
			Status:             Undefine,
		})
		return nil
	})
}

// Store `dealInfo` in the PieceStore with key `pieceCID`.
func (ps *dsPieceStore) UpdateDealOnPacking(pieceCID cid.Cid, dealId abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error {
	return ps.mutatePieceInfo(pieceCID, func(pi *PieceInfo) error {
		for _, di := range pi.Deals {
			if di.DealID == dealId {
				di.SectorID = sectorid
				di.Offset = offset
				di.Status = Assigned
				return nil
			}
		}
		//new deal
		return nil
	})
}

// Store `dealInfo` in the PieceStore with key `pieceCID`.
func (ps *dsPieceStore) UpdateDealStatus(dealId abi.DealID, status string) error {
	return ps.mutateDeal(func(info *DealInfo) (bool, error) {
		if info.DealID == dealId {
			info.Status = status
			return false, nil
		}
		return true, nil
	})
}

func (ps *dsPieceStore) GetDealByPosition(ctx context.Context, sid abi.SectorID, offset abi.PaddedPieceSize, length abi.PaddedPieceSize) (*DealInfo, error) {
	var dinfo *DealInfo
	err := ps.eachPackedDeal(func(info *DealInfo) (bool, error) {
		if info.SectorID == sid.Number && info.Offset <= offset && info.Offset+info.Length >= offset+length {
			dinfo = info
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if dinfo == nil {
		return nil, xerrors.Errorf("unable to find deal position, maybe deal not ready")
	}
	return dinfo, nil
}

func (ps *dsPieceStore) GetDeals(pageIndex, pageSize int) ([]*DealInfo, error) {
	var deals []*DealInfo
	count := 0
	from := pageIndex * pageSize
	to := (pageIndex + 1) * pageSize
	err := ps.eachDeal(func(info *DealInfo) (bool, error) {
		if count < from {
			return true, nil
		} else if count > to {
			return false, nil
		} else {
			deals = append(deals, info)
			return true, nil
		}
	})
	if err != nil {
		return nil, err
	}
	return deals, nil
}

var defaultMaxPiece = 10
var defaultGetDealSpec = &GetDealSpec{
	MaxPiece:     defaultMaxPiece,
	MaxPieceSize: 0,
}

func (ps *dsPieceStore) AssignUnPackedDeals(spec *GetDealSpec) ([]*DealInfoIncludePath, error) {
	deals, err := ps.GetUnPackedDeals(&GetDealSpec{MaxPiece: math.MaxInt32}) //todo get all pending deals
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
	sectorCap := abi.PaddedPieceSize(ps.ssize).Unpadded()

	// 按尺寸分组
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
	for _, p := range pieces {
		err := ps.mutateDeal(func(info *DealInfo) (bool, error) {
			if info.DealID == p.DealID {
				info.Status = Assigned
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return nil, err
		}
	}
	return pieces, nil
}

func (ps *dsPieceStore) GetUnPackedDeals(spec *GetDealSpec) ([]*DealInfoIncludePath, error) {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	if spec == nil {
		spec = defaultGetDealSpec
	}
	if spec.MaxPiece == 0 {
		spec.MaxPiece = defaultMaxPiece
	}

	qres, err := ps.pieces.Query(query.Query{})
	if err != nil {
		return nil, xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	var result []*DealInfoIncludePath
	var curPiece int
	var curPieceSize uint64
LOOP:
	for r := range qres.Next() {
		var pieceInfo PieceInfo
		err := json.Unmarshal(r.Value, &pieceInfo)
		if err != nil {
			return nil, xerrors.Errorf("unable to parser cid: %w", err)
		}

		for _, deal := range pieceInfo.Deals {
			if deal.Status == Undefine {
				result = append(result, &DealInfoIncludePath{
					DealProposal:    deal.Proposal,
					Offset:          deal.Offset,
					Length:          deal.Length,
					DealID:          deal.DealID,
					TotalStorageFee: deal.Proposal.TotalStorageFee(),
					PieceStorage:    path.Join(string(*ps.pieceStorage), deal.Proposal.PieceCID.String()),
					FastRetrieval:   deal.FastRetrieval,
					PublishCid:      deal.PublishCid,
				})
				deal.Status = Assigned

				curPiece++
				curPieceSize += uint64(deal.Length)
				if spec.MaxPiece > 0 && curPiece > spec.MaxPiece {
					goto LOOP
				}
				if spec.MaxPieceSize > 0 && curPieceSize > spec.MaxPieceSize {
					goto LOOP
				}
			}
		}
	}

	return result, nil
}

func (ps *dsPieceStore) MarkDealsAsPacking(deals []abi.DealID) error {
	pieces, err := ps.ListPieceInfoKeys()
	if err != nil {
		return err
	}

	for _, piece := range pieces {
		err = ps.mutatePieceInfo(piece, func(pi *PieceInfo) error {
			for _, deal := range pi.Deals {
				for _, inDeal := range deals {
					if deal.DealID == inDeal {
						deal.Status = Assigned
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ps *dsPieceStore) ListPieceInfoKeys() ([]cid.Cid, error) {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	qres, err := ps.pieces.Query(query.Query{})
	if err != nil {
		return nil, xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	var out []cid.Cid
	for r := range qres.Next() {
		id, err := cid.Decode(strings.TrimPrefix(r.Key, "/"))
		if err != nil {
			return nil, xerrors.Errorf("unable to parser cid: %w", err)
		}
		out = append(out, id)
	}

	return out, nil
}

// Retrieve the PieceInfo associated with `pieceCID` from the piece info store.
func (ps *dsPieceStore) GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error) {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	key := datastore.NewKey(pieceCID.String())
	pieceBytes, err := ps.pieces.Get(key)
	if err != nil {
		return piecestore.PieceInfo{}, err
	}
	piInfo := piecestore.PieceInfo{}
	if err = json.Unmarshal(pieceBytes, &piInfo); err != nil {
		return piecestore.PieceInfo{}, err
	}
	piInfo.PieceCID = pieceCID
	return piInfo, nil
}

func (ps *dsPieceStore) mutatePieceInfo(pieceCID cid.Cid, mutator func(pi *PieceInfo) error) error {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()
	key := datastore.NewKey(pieceCID.String())
	pieceBytes, err := ps.pieces.Get(key)
	if err != nil && datastore.ErrNotFound != err {
		return err
	}

	piInfo := PieceInfo{}
	if pieceBytes != nil {
		if err = json.Unmarshal(pieceBytes, &piInfo); err != nil {
			return err
		}
	}

	if err = mutator(&piInfo); err != nil {
		return err
	}
	data, err := json.Marshal(piInfo)
	if err != nil {
		return err
	}
	return ps.pieces.Put(key, data)
}

func (ps *dsPieceStore) eachPackedDeal(f func(info *DealInfo) (bool, error)) error {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	qres, err := ps.pieces.Query(query.Query{})
	if err != nil {
		return xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	for r := range qres.Next() {
		var pieceInfo PieceInfo
		err := json.Unmarshal(r.Value, &pieceInfo)
		if err != nil {
			return xerrors.Errorf("unable to parser cid: %w", err)
		}

		for _, deal := range pieceInfo.Deals {
			if deal.Status != Undefine {
				isContinue, err := f(deal)
				if err != nil {
					return err
				}
				if !isContinue {
					break
				}
			}
		}
	}

	return nil
}

func (ps *dsPieceStore) eachDeal(f func(info *DealInfo) (bool, error)) error {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	qres, err := ps.pieces.Query(query.Query{})
	if err != nil {
		return xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	for r := range qres.Next() {
		var pieceInfo PieceInfo
		err := json.Unmarshal(r.Value, &pieceInfo)
		if err != nil {
			return xerrors.Errorf("unable to parser cid: %w", err)
		}

		for _, deal := range pieceInfo.Deals {
			isContinue, err := f(deal)
			if err != nil {
				return err
			}
			if !isContinue {
				break
			}
		}
	}

	return nil
}

func (ps *dsPieceStore) mutateDeal(f func(info *DealInfo) (bool, error)) error {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	qres, err := ps.pieces.Query(query.Query{})
	if err != nil {
		return xerrors.Errorf("query error: %w", err)
	}

	modify := map[cid.Cid]PieceInfo{}
	for r := range qres.Next() {
		id, err := cid.Decode(strings.TrimPrefix(r.Key, "/"))
		if err != nil {
			_ = qres.Close()
			return xerrors.Errorf("unable to parser cid: %w", err)
		}
		var pieceInfo PieceInfo
		err = json.Unmarshal(r.Value, &pieceInfo)
		if err != nil {
			_ = qres.Close()
			return xerrors.Errorf("unable to parser pieceinfo: %w", err)
		}

		for _, deal := range pieceInfo.Deals {
			isContinue, err := f(deal)
			if err != nil {
				_ = qres.Close()
				return err
			}
			if !isContinue {
				break
			}
		}
		modify[id] = pieceInfo
		//todo poor performance
	}

	_ = qres.Close()

	for pieceCid, pieceInfo := range modify {
		data, err := json.Marshal(pieceInfo)
		if err != nil {
			return err
		}

		err = ps.pieces.Put(datastore.NewKey(pieceCid.String()), data)
		if err != nil {
			return err
		}
	}
	return nil
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
		psize := uint64(1) << next
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
