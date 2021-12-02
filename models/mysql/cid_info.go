package mysql

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cidInfoTableName = "cid_infos"

type mysqlCidInfoRepo struct {
	ds *gorm.DB
}

var _ repo.ICidInfoRepo = (*mysqlCidInfoRepo)(nil)

func NewMysqlCidInfoRepo(ds *gorm.DB) *mysqlCidInfoRepo {
	return &mysqlCidInfoRepo{ds: ds}
}

type mysqlBlockLocation piecestore.BlockLocation

func (mbl *mysqlBlockLocation) Scan(value interface{}) error {
	var a, ok = value.([]byte)
	if !ok {
		return errors.New("address should be a string")
	}
	return json.Unmarshal(a, mbl)
}

func (mbl mysqlBlockLocation) Value() (driver.Value, error) {
	return json.Marshal(mbl)
}

type cidInfo struct {
	PieceCid      DBCid              `gorm:"column:piece_cid;primaryKey;type:varchar(256)"`
	PayloadCid    DBCid              `gorm:"column:payload_cid;primaryKey;type:varchar(256);"`
	BlockLocation mysqlBlockLocation `gorm:"block_location;type:json"`
	TimeStampOrm
}

func (m cidInfo) TableName() string {
	return cidInfoTableName
}

func (m *mysqlCidInfoRepo) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	mysqlInfos := make([]cidInfo, len(blockLocations))
	idx := 0
	for blockCid, location := range blockLocations {
		mysqlInfos[idx].PieceCid = DBCid(pieceCID)
		mysqlInfos[idx].PayloadCid = DBCid(blockCid)
		mysqlInfos[idx].BlockLocation = mysqlBlockLocation(location)
		mysqlInfos[idx].UpdatedAt = uint64(time.Now().Unix())
		idx++
	}

	return m.ds.Table(cidInfoTableName).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "piece_cid"}, {Name: "payload_cid"}},
		UpdateAll: true,
	}).Save(mysqlInfos).Error

}

func (m *mysqlCidInfoRepo) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	cidInfo := cidInfo{}
	if err := m.ds.Model(&cidInfo).Find(&cidInfo, "payload_cid = ?", DBCid(payloadCID).String()).Error; err != nil {
		return piecestore.CIDInfo{}, err
	}
	return piecestore.CIDInfo{
		CID: payloadCID,
		PieceBlockLocations: []piecestore.PieceBlockLocation{
			{BlockLocation: piecestore.BlockLocation(cidInfo.BlockLocation),
				PieceCID: cid.Cid(cidInfo.PieceCid),
			},
		}}, nil
}

func (m *mysqlCidInfoRepo) ListCidInfoKeys() ([]cid.Cid, error) {
	var cidsStr []string
	err := m.ds.Table(cidInfoTableName).Select("payload_cid").Scan(&cidsStr).Error
	if err != nil {
		return nil, err
	}
	cids := make([]cid.Cid, len(cidsStr))
	for idx, s := range cidsStr {
		cids[idx], err = cid.Decode(s)
		if err != nil {
			return nil, err
		}
	}

	return cids, nil

}

var _ repo.ICidInfoRepo = (*mysqlCidInfoRepo)(nil)
