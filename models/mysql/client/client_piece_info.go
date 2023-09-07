package client

import (
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

const pieceInfoTableName = "piece_infos"

type pieceInfo struct {
	ID          uint64      `gorm:"column:id;autoIncrement;primary_key"`
	PieceCID    mysql.DBCid `gorm:"column:piece_cid;type:varchar(256);uniqueIndex"`
	PieceSize   uint64      `gorm:"column:piece_size;type:bigint unsigned;NOT NULL"`
	PayloadCID  mysql.DBCid `gorm:"column:payload_cid;type:varchar(256);NOT NULL"`
	PayloadSize uint64      `gorm:"column:payload_size;type:bigint unsigned;NOT NULL"`
}

func (pi *pieceInfo) TableName() string {
	return pieceInfoTableName
}

func (pi *pieceInfo) toPieceInfo() *client.ClientPieceInfo {
	return &client.ClientPieceInfo{
		ID:          pi.ID,
		PieceCID:    cid.Cid(pi.PieceCID),
		PieceSize:   pi.PieceSize,
		PayloadCID:  cid.Cid(pi.PayloadCID),
		PayloadSize: pi.PayloadSize,
	}
}

func fromPieceInfo(pi *client.ClientPieceInfo) *pieceInfo {
	return &pieceInfo{
		ID:          pi.ID,
		PieceCID:    mysql.DBCid(pi.PieceCID),
		PieceSize:   pi.PieceSize,
		PayloadCID:  mysql.DBCid(pi.PayloadCID),
		PayloadSize: pi.PayloadSize,
	}
}

type clientPieceInfoRepo struct {
	*gorm.DB
}

var _ ClientPieceInfoRepo = (*clientPieceInfoRepo)(nil)

func NewClientPieceInfoRepo(db *gorm.DB) ClientPieceInfoRepo {
	return &clientPieceInfoRepo{db}
}

func (pir *clientPieceInfoRepo) SavePieceInfo(pi *client.ClientPieceInfo) error {
	return pir.DB.Save(fromPieceInfo(pi)).Error
}

func (pir *clientPieceInfoRepo) GetPieceInfo(pieceCID cid.Cid) (*client.ClientPieceInfo, error) {
	var pi pieceInfo
	if err := pir.DB.Take(&pi, "piece_cid = ?", pieceCID.String()).Error; err != nil {
		return nil, err
	}

	return pi.toPieceInfo(), nil
}

func (pir *clientPieceInfoRepo) ListPieceInfo() ([]*client.ClientPieceInfo, error) {
	var pis []*pieceInfo
	if err := pir.DB.Find(&pis).Error; err != nil {
		return nil, err
	}
	out := make([]*client.ClientPieceInfo, 0, len(pis))
	for _, pi := range pis {
		out = append(out, pi.toPieceInfo())
	}

	return out, nil
}
