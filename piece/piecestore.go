package piece

import (
	"context"
	"encoding/json"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/models"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
	"sync"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared"
)

var log = logging.Logger("piecestore")

// DSPiecePrefix is the name space for storing piece infos
var DSPiecePrefix = "/pieces"

// DSCIDPrefix is the name space for storing CID infos
var DSCIDPrefix = "/cid-infos"

type CIDInfo struct {
	piecestore.CIDInfo
}

type PieceInfo struct {
	PieceCID cid.Cid
	Deals    []DealInfo
}

type DealInfo struct {
	piecestore.DealInfo
	market.ClientDealProposal
	TransferType  string
	Root          cid.Cid
	PublishCid    cid.Cid
	DealId        abi.DealID
	FastRetrieval bool
	IsPacking     bool
}

type GetDealSpec struct {
	MaxNumber int
}

type ExtendPieceStore interface {
	piecestore.PieceStore

	GetUnPackedDeals(spec *GetDealSpec) ([]DealInfo, error)
	MarkDealsAsPacking(deals []abi.DealID) error
	UpdateDealOnComplete(pieceCID cid.Cid, proposal market.ClientDealProposal, dataRef *storagemarket.DataRef, publishCid cid.Cid, dealId abi.DealID, fastRetrieval bool) error
	UpdateDealOnPacking(pieceCID cid.Cid, dealId abi.DealID, sectorid abi.SectorNumber, offset, length abi.PaddedPieceSize) error
}

var _ ExtendPieceStore = (*dsPieceStore)(nil)

var _ piecestore.PieceStore = (*dsPieceStore)(nil)

type dsPieceStore struct {
	pieces   datastore.Batching
	cidInfos datastore.Batching

	pieceLk sync.Mutex
}

// NewDsPieceStore returns a new piecestore based on the given datastore
func NewDsPieceStore(ds models.PieceMetaDs) (ExtendPieceStore, error) {
	return &dsPieceStore{
		pieces:   namespace.Wrap(ds, datastore.NewKey(DSPiecePrefix)),
		cidInfos: namespace.Wrap(ds, datastore.NewKey(DSCIDPrefix)),
		pieceLk:  sync.Mutex{},
	}, nil
}

func (ps *dsPieceStore) Start(ctx context.Context) error {
	return nil
}

func (ps *dsPieceStore) OnReady(ready shared.ReadyFunc) {
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
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	return ps.mutatePieceInfo(pieceCID, func(pi *PieceInfo) error {
		for _, di := range pi.Deals {
			if di.DealID == dealId {
				return nil
			}
		}
		//new deal
		pi.Deals = append(pi.Deals, DealInfo{
			DealInfo: piecestore.DealInfo{
				DealID:   dealId,
				SectorID: 0,
				Offset:   0,
				Length:   proposal.Proposal.PieceSize,
			},
			ClientDealProposal: market.ClientDealProposal{},
			TransferType:       dataRef.TransferType,
			Root:               dataRef.Root,
			PublishCid:         publishCid,
			DealId:             dealId,
			FastRetrieval:      fastRetrieval,
			IsPacking:          false,
		})
		return nil
	})
}

// Store `dealInfo` in the PieceStore with key `pieceCID`.
func (ps *dsPieceStore) UpdateDealOnPacking(pieceCID cid.Cid, dealId abi.DealID, sectorid abi.SectorNumber, offset, length abi.PaddedPieceSize) error {
	return ps.mutatePieceInfo(pieceCID, func(pi *PieceInfo) error {
		for _, di := range pi.Deals {
			if di.DealID == dealId {
				di.SectorID = sectorid
				di.Offset = offset
				di.IsPacking = true
				return nil
			}
		}
		//new deal
		return nil
	})
}

func (ps *dsPieceStore) GetUnPackedDeals(spec *GetDealSpec) ([]DealInfo, error) {
	ps.pieceLk.Lock()
	defer ps.pieceLk.Unlock()

	qres, err := ps.pieces.Query(query.Query{})
	if err != nil {
		return nil, xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	var result []DealInfo
	for r := range qres.Next() {
		var pieceInfo PieceInfo
		err := json.Unmarshal(r.Value, &pieceInfo)
		if err != nil {
			return nil, xerrors.Errorf("unable to parser cid: %w", err)
		}

		for _, deal := range pieceInfo.Deals {
			if !deal.IsPacking {
				result = append(result, deal)
			}
		}
	}

	return result, nil
}

func (ps *dsPieceStore) MarkDealsAsPacking(deals []abi.DealID) error {
	pieces, err := ps.ListCidInfoKeys()
	if err != nil {
		return err
	}

	for _, piece := range pieces {
		err = ps.mutatePieceInfo(piece, func(pi *PieceInfo) error {
			for _, deal := range pi.Deals {
				for _, inDeal := range deals {
					if deal.DealId == inDeal {
						deal.IsPacking = true
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

// Store the map of blockLocations in the PieceStore's CIDInfo store, with key `pieceCID`
func (ps *dsPieceStore) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	for c, blockLocation := range blockLocations {
		err := ps.mutateCIDInfo(c, func(ci *CIDInfo) error {
			for _, pbl := range ci.PieceBlockLocations {
				if pbl.PieceCID.Equals(pieceCID) && pbl.BlockLocation == blockLocation {
					return nil
				}
			}
			ci.PieceBlockLocations = append(ci.PieceBlockLocations, piecestore.PieceBlockLocation{BlockLocation: blockLocation, PieceCID: pieceCID})
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
		id, err := cid.Decode(r.Key)
		if err != nil {
			return nil, xerrors.Errorf("unable to parser cid: %w", err)
		}
		out = append(out, id)
	}

	return out, nil
}

func (ps *dsPieceStore) ListCidInfoKeys() ([]cid.Cid, error) {
	qres, err := ps.cidInfos.Query(query.Query{})
	if err != nil {
		return nil, xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	var out []cid.Cid
	for r := range qres.Next() {
		id, err := cid.Decode(r.Key)
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
	return piInfo, nil
}

// Retrieve the CIDInfo associated with `pieceCID` from the CID info store.
func (ps *dsPieceStore) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	key := datastore.NewKey(payloadCID.String())
	cidInfoBytes, err := ps.pieces.Get(key)
	if err != nil {
		return piecestore.CIDInfo{}, err
	}
	cidInfo := piecestore.CIDInfo{}
	if err = json.Unmarshal(cidInfoBytes, &cidInfo); err != nil {
		return piecestore.CIDInfo{}, err
	}
	return cidInfo, nil
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

func (ps *dsPieceStore) mutateCIDInfo(c cid.Cid, mutator func(ci *CIDInfo) error) error {
	key := datastore.NewKey(c.String())
	cidInfoBytes, err := ps.pieces.Get(key)
	if err != nil && datastore.ErrNotFound != err {
		return err
	}

	cidInfo := CIDInfo{}
	if cidInfoBytes == nil {
		if err = json.Unmarshal(cidInfoBytes, &cidInfo); err != nil {
			return err
		}
	}

	if err = mutator(&cidInfo); err != nil {
		return err
	}

	data, err := json.Marshal(cidInfo)
	if err != nil {
		return err
	}
	return ps.pieces.Put(key, data)
}
