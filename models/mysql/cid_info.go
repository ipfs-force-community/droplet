package mysql

import (
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type mysqlCidInfoRepo struct {
	ds *gorm.DB
}

var _ repo.ICidInfoRepo = (*mysqlCidInfoRepo)(nil)

func NewMysqlCidInfoRepo(ds *gorm.DB) *mysqlCidInfoRepo {
	return &mysqlCidInfoRepo{ds: ds}
}

func (m *mysqlCidInfoRepo) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	panic("implement me")
}

func (m *mysqlCidInfoRepo) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	panic("implement me")
}

func (m *mysqlCidInfoRepo) ListCidInfoKeys() ([]cid.Cid, error) {
	panic("implement me")
}

var _ repo.ICidInfoRepo = (*mysqlCidInfoRepo)(nil)
