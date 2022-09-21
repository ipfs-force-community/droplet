package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"sort"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cidInfoTableName = "cid_infos"

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

type mysqlCidInfoRepo struct {
	*gorm.DB
}

var _ repo.ICidInfoRepo = (*mysqlCidInfoRepo)(nil)

func NewMysqlCidInfoRepo(ds *gorm.DB) repo.ICidInfoRepo {
	return &mysqlCidInfoRepo{ds}
}

func (m *mysqlCidInfoRepo) AddPieceBlockLocations(ctx context.Context, pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	mysqlInfos := make([]cidInfo, len(blockLocations))
	idx := 0
	/* following point has been confirmed :
		there is no needs to worry about `create_at`/`updated_at`,
	 because both of them are 0, the `gorm` framework deal well of them automatically. */
	for blockCid, location := range blockLocations {
		mysqlInfos[idx].PieceCid = DBCid(pieceCID)
		mysqlInfos[idx].PayloadCid = DBCid(blockCid)
		mysqlInfos[idx].BlockLocation = mysqlBlockLocation(location)
		idx++
	}

	// make the order of sql predictable
	sort.Slice(mysqlInfos, func(i, j int) bool {
		return mysqlInfos[i].PayloadCid.String() < mysqlInfos[j].PayloadCid.String()
	})

	return m.Table(cidInfoTableName).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "piece_cid"}, {Name: "payload_cid"}},
		UpdateAll: true,
	}).Save(mysqlInfos).Error

}

func (m *mysqlCidInfoRepo) GetCIDInfo(ctx context.Context, payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	cidInfo := cidInfo{}
	if err := m.Model(&cidInfo).Find(&cidInfo, "payload_cid = ?", DBCid(payloadCID).String()).Error; err != nil {
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

func (m *mysqlCidInfoRepo) ListCidInfoKeys(ctx context.Context) ([]cid.Cid, error) {
	var cidsStr []string
	err := m.Table(cidInfoTableName).Select("payload_cid").Scan(&cidsStr).Error
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
