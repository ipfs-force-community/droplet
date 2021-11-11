package mysql

import (
	"database/sql/driver"
	"encoding/json"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mysqlCidInfoRepo struct {
	ds *gorm.DB
}

var _ repo.ICidInfoRepo = (*mysqlCidInfoRepo)(nil)

func NewMysqlCidInfoRepo(ds *gorm.DB) *mysqlCidInfoRepo {
	return &mysqlCidInfoRepo{ds: ds}
}

type mysqlCid cid.Cid

func (mc *mysqlCid) Scan(value interface{}) error {
	var a, ok = value.([]byte)
	if !ok {
		return errors.New("address should be a string")
	}
	id, err := cid.Decode(string(a))
	if err != nil {
		return err
	}
	*mc = mysqlCid(id)
	return nil
}

func (mc mysqlCid) Value() (driver.Value, error) {
	return ((cid.Cid)(mc)).String(), nil
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
	PieceCid      mysqlCid           `gorm:"column:piece_cid;primaryKey;type:varchar(128)"`
	PayloadCid    mysqlCid           `gorm:"column:payload_cid;primaryKey;type:varchar(128);index"`
	BlockLocation mysqlBlockLocation `gorm:"block_location;type:json"`
	TimeStampOrm
}

func (m cidInfo) TableName() string {
	return "cid_infos"
}

func (m *mysqlCidInfoRepo) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {

	mysqlInfos := make([]cidInfo, len(blockLocations))
	idx := 0
	for blockCid, location := range blockLocations {
		mysqlInfos[idx].PieceCid = mysqlCid(pieceCID)
		mysqlInfos[idx].PayloadCid = mysqlCid(blockCid)
		mysqlInfos[idx].BlockLocation = mysqlBlockLocation(location)
		idx++
	}

	return m.ds.Table("cid_infos").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "piece_cid"}, {Name: "block_cid"}},
		UpdateAll: true,
	}).Save(mysqlInfos).Error

}

func (m *mysqlCidInfoRepo) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	cidInfo := cidInfo{}
	if err := m.ds.Model(&cidInfo).Find(&cidInfo, "payload_cid = ?", payloadCID.String()).Error; err != nil {
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

func (m *mysqlCidInfoRepo) ListCidInfoKeys() (cids []cid.Cid, err error) {
	var cidsStr []string
	defer func() {
		size := len(cidsStr)
		if size == 0 {
			return
		}
		cids = make([]cid.Cid, size)
		for idx, s := range cidsStr {
			cids[idx], _ = cid.Decode(s)
		}
	}()
	return cids, m.ds.Table((cidInfo{}).TableName()).Select("payload_cid").Scan(&cidsStr).Error

}

func (m *mysqlCidInfoRepo) Close() error {
	db, err := m.ds.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
