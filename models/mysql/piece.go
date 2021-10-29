package mysql

import (
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type mysqlPieceRepo struct {
	ds *gorm.DB
}

var _ repo.IPieceRepo = (*mysqlPieceRepo)(nil)

func NewMysqlPieceRepo(ds *gorm.DB) *mysqlPieceRepo {
	return &mysqlPieceRepo{ds: ds}
}

func (m *mysqlPieceRepo) AddDealForPiece(pieceCID cid.Cid, dealInfo piecestore.DealInfo) error {
	panic("implement me")
}

func (m *mysqlPieceRepo) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	panic("implement me")
}

func (m *mysqlPieceRepo) GetPieceInfo(pieceCID cid.Cid) (piecestore.PieceInfo, error) {
	panic("implement me")
}

func (m *mysqlPieceRepo) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	panic("implement me")
}

func (m *mysqlPieceRepo) ListCidInfoKeys() ([]cid.Cid, error) {
	panic("implement me")
}

func (m *mysqlPieceRepo) ListPieceInfoKeys() ([]cid.Cid, error) {
	panic("implement me")
}

var _ repo.IPieceRepo = (*mysqlPieceRepo)(nil)
