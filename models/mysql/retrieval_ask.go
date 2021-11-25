package mysql

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models/repo"
	mtypes "github.com/filecoin-project/venus-messager/types"
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

type modelRetrievalAsk struct {
	ID                      uint       `gorm:"primary_key"`
	Address                 DBAddress  `gorm:"column:address;uniqueIndex;type:varchar(128)"`
	PricePerByte            mtypes.Int `gorm:"column:price_per_byte;type:varchar(256);"`
	UnsealPrice             mtypes.Int `gorm:"column:unseal_price;type:varchar(256);"`
	PaymentInterval         uint64     `gorm:"column:payment_interval;type:bigint unsigned;"`
	PaymentIntervalIncrease uint64     `gorm:"column:payment_interval_increase;type:bigint unsigned;"`
	TimeStampOrm
}

func (a *modelRetrievalAsk) TableName() string {
	return "retrieval_asks"
}

func (r *retrievalAskRepo) GetAsk(addr address.Address) (*retrievalmarket.Ask, error) {
	var mAsk modelRetrievalAsk
	if err := r.ds.Take(&mAsk, "address = ?", DBAddress(addr).String()).Error; err != nil {
		if xerrors.Is(err, gorm.ErrRecordNotFound) {
			err = repo.ErrNotFound
		}
		return nil, err
	}
	return &retrievalmarket.Ask{
		PricePerByte:            fbig.Int{Int: mAsk.PricePerByte.Int},
		UnsealPrice:             fbig.Int{Int: mAsk.UnsealPrice.Int},
		PaymentInterval:         mAsk.PaymentInterval,
		PaymentIntervalIncrease: mAsk.PaymentIntervalIncrease,
	}, nil
}

func (r *retrievalAskRepo) SetAsk(addr address.Address, ask *retrievalmarket.Ask) error {
	return r.ds.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		UpdateAll: true,
	}).Save(&modelRetrievalAsk{
		Address:                 DBAddress(addr),
		PricePerByte:            convertBigInt(ask.PricePerByte),
		UnsealPrice:             convertBigInt(ask.UnsealPrice),
		PaymentInterval:         ask.PaymentInterval,
		PaymentIntervalIncrease: ask.PaymentIntervalIncrease,
	}).Error
}
