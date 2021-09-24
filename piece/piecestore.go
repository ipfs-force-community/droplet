package piece

import (
	"context"
	"encoding/json"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models"
	"path"
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
	DealId        abi.DealID
	FastRetrieval bool
	Status        string
}

type DealInfoIncludePath struct {
	DealInfo
	PieceStorage string
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
}

// NewDsPieceStore returns a new piecestore based on the given datastore
func NewDsPieceStore(ds models.PieceInfoDS, pieceStorage *config.PieceStorageString) (PieceStore, error) {
	return &dsPieceStore{
		pieces:       ds,
		pieceStorage: pieceStorage,
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
			DealId:             dealId,
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

	var result []*DealInfoIncludePath
	var curPiece int
	var curPieceSize uint64
	var modify map[string]PieceInfo
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
					DealInfo:     *deal,
					PieceStorage: path.Join(string(*ps.pieceStorage), deal.Proposal.PieceCID.String()),
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
		modify[r.Key] = pieceInfo
	}

	err = qres.Close()
	if err != nil {
		return nil, xerrors.Errorf("close badger close %w", err)
	}

	for key, val := range modify {
		vBytes, err := json.Marshal(val)
		if err != nil {
			return nil, xerrors.Errorf("marshal piece info %w", err)
		}
		err = ps.pieces.Put(datastore.RawKey(key), vBytes)
		if err != nil {
			return nil, xerrors.Errorf("update piece info %w", err)
		}
	}
	return result, nil
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
					DealInfo:     *deal,
					PieceStorage: path.Join(string(*ps.pieceStorage), deal.Proposal.PieceCID.String()),
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
					if deal.DealId == inDeal {
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
