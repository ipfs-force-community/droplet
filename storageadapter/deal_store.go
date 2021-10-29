package storageadapter

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
)

var RecordNotFound = fmt.Errorf("unable to find record")

type StorageDealStore interface {
	SaveDeal(deal *storagemarket.MinerDeal) error
	GetDeal(cid cid.Cid) (*storagemarket.MinerDeal, error)
	List(mAddr address.Address, out interface{}) error
	GetPieceInfoFromCid(ctx context.Context, payloadCID, pieceCID cid.Cid) (piecestore.PieceInfo, bool, error)
	GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error)
}

// TODO: create instance
