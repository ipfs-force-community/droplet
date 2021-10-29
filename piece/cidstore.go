package piece

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"golang.org/x/xerrors"
)

type CIDInfo struct {
	piecestore.CIDInfo
}

type CIDStore interface {
	AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error
	ListCidInfoKeys() ([]cid.Cid, error)
	GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error)
}

type dsCidInfoStore struct {
	cidInfos datastore.Batching
	pieceLk  sync.Mutex
}

// NewDsPieceStore returns a new piecestore based on the given datastore
func NewDsCidInfoStore(ds repo.CIDInfoDS) (CIDStore, error) {
	return &dsCidInfoStore{
		cidInfos: ds,
		pieceLk:  sync.Mutex{},
	}, nil
}

// Store the map of blockLocations in the PieceStore's CIDInfo store, with key `pieceCID`
func (ps *dsCidInfoStore) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	for c, blockLocation := range blockLocations {
		err := ps.mutateCIDInfo(c, func(ci *CIDInfo) error {
			for _, pbl := range ci.PieceBlockLocations {
				if pbl.PieceCID.Equals(pieceCID) && pbl.BlockLocation == blockLocation {
					return nil
				}
			}

			ci.CID = pieceCID
			ci.PieceBlockLocations = append(ci.PieceBlockLocations, piecestore.PieceBlockLocation{BlockLocation: blockLocation, PieceCID: pieceCID})
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

////********CIDINFO*********
func (ps *dsCidInfoStore) ListCidInfoKeys() ([]cid.Cid, error) {
	qres, err := ps.cidInfos.Query(query.Query{})
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

// Retrieve the CIDInfo associated with `pieceCID` from the CID info store.
func (ps *dsCidInfoStore) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	key := datastore.NewKey(payloadCID.String())
	cidInfoBytes, err := ps.cidInfos.Get(key)
	if err != nil {
		return piecestore.CIDInfo{}, err
	}
	cidInfo := piecestore.CIDInfo{}
	if err = json.Unmarshal(cidInfoBytes, &cidInfo); err != nil {
		return piecestore.CIDInfo{}, err
	}
	cidInfo.CID = payloadCID
	return cidInfo, nil
}

func (ps *dsCidInfoStore) mutateCIDInfo(c cid.Cid, mutator func(ci *CIDInfo) error) error {
	key := datastore.NewKey(c.String())
	cidInfoBytes, err := ps.cidInfos.Get(key)
	if err != nil && datastore.ErrNotFound != err {
		return err
	}

	cidInfo := CIDInfo{}
	if cidInfoBytes != nil {
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
	return ps.cidInfos.Put(key, data)
}
