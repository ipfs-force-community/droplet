package piece

import (
	"bytes"
	"context"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared"
)

var log = logging.Logger("piecestore")

// DSPiecePrefix is the name space for storing piece infos
var DSPiecePrefix = "/pieces"

// DSCIDPrefix is the name space for storing CID infos
var DSCIDPrefix = "/cid-infos"

// NewPieceStore returns a new piecestore based on the given datastore
func NewPieceStore(ds datastore.Batching) (piecestore.PieceStore, error) {
	return &pieceStore{
		pieces:   namespace.Wrap(ds, datastore.NewKey(DSPiecePrefix)),
		cidInfos: namespace.Wrap(ds, datastore.NewKey(DSCIDPrefix)),
	}, nil
}

type pieceStore struct {
	pieces   datastore.Batching
	cidInfos datastore.Batching
}

func (ps *pieceStore) Start(ctx context.Context) error {
	return nil
}

func (ps *pieceStore) OnReady(ready shared.ReadyFunc) {
}

// Store `dealInfo` in the PieceStore with key `pieceCID`.
func (ps *pieceStore) AddDealForPiece(pieceCID cid.Cid, dealInfo piecestore.DealInfo) error {
	return ps.mutatePieceInfo(pieceCID, func(pi *piecestore.PieceInfo) error {
		for _, di := range pi.Deals {
			if di == dealInfo {
				return nil
			}
		}
		pi.Deals = append(pi.Deals, dealInfo)
		return nil
	})
}

// Store the map of blockLocations in the PieceStore's CIDInfo store, with key `pieceCID`
func (ps *pieceStore) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	for c, blockLocation := range blockLocations {
		err := ps.mutateCIDInfo(c, func(ci *piecestore.CIDInfo) error {
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

func (ps *pieceStore) ListPieceInfoKeys() ([]cid.Cid, error) {
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

func (ps *pieceStore) ListCidInfoKeys() ([]cid.Cid, error) {
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
func (ps *pieceStore) GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error) {
	key := datastore.NewKey(pieceCID.String())
	pieceBytes, err := ps.pieces.Get(key)
	if err != nil {
		return piecestore.PieceInfo{}, err
	}
	piInfo := piecestore.PieceInfo{}
	if err = piInfo.UnmarshalCBOR(bytes.NewReader(pieceBytes)); err != nil {
		return piecestore.PieceInfo{}, err
	}
	return piInfo, nil
}

// Retrieve the CIDInfo associated with `pieceCID` from the CID info store.
func (ps *pieceStore) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	key := datastore.NewKey(payloadCID.String())
	cidInfoBytes, err := ps.pieces.Get(key)
	if err != nil {
		return piecestore.CIDInfo{}, err
	}
	cidInfo := piecestore.CIDInfo{}
	if err = cidInfo.UnmarshalCBOR(bytes.NewReader(cidInfoBytes)); err != nil {
		return piecestore.CIDInfo{}, err
	}
	return cidInfo, nil
}

func (ps *pieceStore) mutatePieceInfo(pieceCID cid.Cid, mutator func(pi *piecestore.PieceInfo) error) error {
	key := datastore.NewKey(pieceCID.String())
	pieceBytes, err := ps.pieces.Get(key)
	if err != nil && datastore.ErrNotFound != err {
		return err
	}

	piInfo := piecestore.PieceInfo{}
	if pieceBytes != nil {
		if err = piInfo.UnmarshalCBOR(bytes.NewReader(pieceBytes)); err != nil {
			return err
		}
	}

	if err = mutator(&piInfo); err != nil {
		return err
	}
	result := bytes.NewBufferString("")
	if err = piInfo.MarshalCBOR(result); err != nil {
		return err
	}
	return ps.pieces.Put(key, result.Bytes())
}

func (ps *pieceStore) mutateCIDInfo(c cid.Cid, mutator func(ci *piecestore.CIDInfo) error) error {
	key := datastore.NewKey(c.String())
	cidInfoBytes, err := ps.pieces.Get(key)
	if err != nil && datastore.ErrNotFound != err {
		return err
	}

	cidInfo := piecestore.CIDInfo{}
	if cidInfoBytes == nil {
		if err = cidInfo.UnmarshalCBOR(bytes.NewReader(cidInfoBytes)); err != nil {
			return err
		}
	}

	if err = mutator(&cidInfo); err != nil {
		return err
	}
	result := bytes.NewBufferString("")
	if err = cidInfo.MarshalCBOR(result); err != nil {
		return err
	}
	return ps.pieces.Put(key, result.Bytes())
}
