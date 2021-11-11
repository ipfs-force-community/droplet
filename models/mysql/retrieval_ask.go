package mysql

import (
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus-market/models/repo"
	"golang.org/x/xerrors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TimeStampOrm struct {
	CreatedAt uint64 `gorm:"type:bigint unsigned"`
	UpdatedAt uint64 `gorm:"type:bigint unsigned"`
	DeleteAt  uint64 `gorm:"type:bigint unsigned;index;default:null"`
}

type retrievalAskRepo struct {
	ds *gorm.DB
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds *gorm.DB) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

type mysqlRetrievalAsk retrievalmarket.Ask
type mysqlAddress address.Address

func (j *mysqlAddress) Scan(value interface{}) error {
	var a, ok = value.([]byte)
	if !ok {
		return errors.New("address should be a string")
	}
	addr, err := address.NewFromString(string(a))
	if err != nil {
		return err
	}
	*j = (mysqlAddress)(addr)
	return nil
}

func (j mysqlAddress) Value() (driver.Value, error) {
	return (address.Address)(j).String(), nil
}

func addressMysqlKey(addr address.Address) string {
	return hex.EncodeToString(addr.Bytes())
}

func (j mysqlAddress) Key() string {
	return addressMysqlKey((address.Address)(j))
}

func (j *mysqlRetrievalAsk) Scan(value interface{}) error {
	var bytes, ok = value.([]byte)
	if !ok {
		return xerrors.New(fmt.Sprint("Failed to unmarshal mysqlAddress value:", value))
	}
	return json.Unmarshal(bytes, j)
}

func (j mysqlRetrievalAsk) Value() (driver.Value, error) {
	return json.Marshal(j)
}

type modelRetrievalAsk struct {
	ID      uint               `gorm:"primary_key"`
	UIdx    string             `gorm:"column:uidx;uniqueIndex;type:varchar(128)"`
	Address mysqlAddress       `gorm:"column:address;uniqueIndex;type:varchar(128)"`
	Ask     *mysqlRetrievalAsk `gorm:"column:retrieval_ask;type:blob;size:2048"`
	TimeStampOrm
}

func (a *modelRetrievalAsk) TableName() string {
	return "retrieval_asks"
}

func (r *retrievalAskRepo) GetAsk(addr address.Address) (*retrievalmarket.Ask, error) {
	var mAsk modelRetrievalAsk
	if err := r.ds.Take(&mAsk, "uidx = ?", (mysqlAddress)(addr).Key()).Error; err != nil {
		if xerrors.Is(err, gorm.ErrRecordNotFound) {
			err = repo.ErrNotFound
		}
		return nil, err
	}
	return (*retrievalmarket.Ask)(mAsk.Ask), nil
}

func (repo *retrievalAskRepo) SetAsk(addr address.Address, ask *retrievalmarket.Ask) error {
	mysqlAddr := (mysqlAddress)(addr)

	return repo.ds.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uidx"}},
		UpdateAll: true,
	}).Save(&modelRetrievalAsk{
		UIdx:    mysqlAddr.Key(),
		Address: mysqlAddr,
		Ask:     (*mysqlRetrievalAsk)(ask),
	}).Error
}