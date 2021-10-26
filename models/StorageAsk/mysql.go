package StorageAsk

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"golang.org/x/xerrors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mysqlStorageAsk struct {
	ds *gorm.DB
}

func init() {
	address.CurrentNetwork = address.Mainnet
}

type mysqlAddress address.Address
type mysqlSignedAsk storagemarket.SignedStorageAsk

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

// Value return json value, implement driver.Valuer interface
func (j mysqlAddress) Value() (driver.Value, error) {
	return (address.Address)(j).String(), nil
}

func (j *mysqlSignedAsk) Scan(value interface{}) error {
	var bytes, ok = value.([]byte)
	if !ok {
		return xerrors.New(fmt.Sprint("Failed to unmarshal mysqlAddress value:", value))
	}
	return json.Unmarshal(bytes, j)
}

// Value return json value, implement driver.Valuer interface
func (j mysqlSignedAsk) Value() (driver.Value, error) {
	return json.Marshal(j)
}

type StAsk struct {
	Miner          mysqlAddress    `gorm:"column:Miner;uniqueIndex;type:varchar(128)"`
	MysqlSignedAsk *mysqlSignedAsk `gorm:"column:SignedAsk;type:blob;size:2048"`
	gorm.Model
}

func (sa *StAsk) Actor() address.Address {
	return address.Address(sa.Miner)
}

func (sa *StAsk) SignedAsk() *storagemarket.SignedStorageAsk {
	return (*storagemarket.SignedStorageAsk)(sa.MysqlSignedAsk)
}

func newMysqlStorageAskRepo(cfg *StorageAskCfg) (*mysqlStorageAsk, error) {
	db, err := gorm.Open(mysql.Open(cfg.URI), &gorm.Config{})
	if err != nil {
		return nil, xerrors.Errorf("new mysql storageask repo failed, open connection failed:%w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err = db.AutoMigrate(&StAsk{}); err != nil {
		return nil, err
	}

	if cfg.Debug {
		db = db.Debug()
	}

	return &mysqlStorageAsk{ds: db}, nil
}

func (b *mysqlStorageAsk) GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	stAsk := &StAsk{Miner: mysqlAddress(miner)}
	if err := b.ds.Model(stAsk).First(stAsk).Error; err != nil {
		return nil, err
	}
	return stAsk.SignedAsk(), nil
}

func (b *mysqlStorageAsk) SetAsk(miner address.Address, ask *storagemarket.SignedStorageAsk) error {
	buf := bytes.NewBuffer(nil)
	if err := ask.MarshalCBOR(buf); err != nil {
		return xerrors.Errorf("bader set Miner(%s) ask, marshal SignedAsk failed:%w",
			miner.String(), err)
	}
	stAsk := &StAsk{Miner: mysqlAddress(miner), MysqlSignedAsk: (*mysqlSignedAsk)(ask)}

	return b.ds.Model(stAsk).Clauses(clause.OnConflict{UpdateAll: true}).Create(stAsk).Error
}

func (b *mysqlStorageAsk) Close() error {
	mysqlDb, err := b.ds.DB()
	if err != nil {
		return err
	}
	return mysqlDb.Close()
}
