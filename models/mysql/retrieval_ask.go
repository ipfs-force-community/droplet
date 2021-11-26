package mysql

import (
	"time"

	"github.com/filecoin-project/venus-market/types"

	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models/repo"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const retrievalAskTableName = "retrieval_asks"

type retrievalAskRepo struct {
	ds *gorm.DB
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(ds *gorm.DB) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{ds: ds}
}

type modelRetrievalAsk struct {
	ID                      uint       `gorm:"primary_key"`
	Address                 DBAddress  `gorm:"column:address;uniqueIndex;type:varchar(256)"`
	PricePerByte            mtypes.Int `gorm:"column:price_per_byte;type:varchar(256);"`
	UnsealPrice             mtypes.Int `gorm:"column:unseal_price;type:varchar(256);"`
	PaymentInterval         uint64     `gorm:"column:payment_interval;type:bigint unsigned;"`
	PaymentIntervalIncrease uint64     `gorm:"column:payment_interval_increase;type:bigint unsigned;"`
	TimeStampOrm
}

func (a *modelRetrievalAsk) TableName() string {
	return retrievalAskTableName
}

func (r *retrievalAskRepo) GetAsk(addr address.Address) (*types.RetrievalAsk, error) {
	var mAsk modelRetrievalAsk
	if err := r.ds.Take(&mAsk, "address = ?", DBAddress(addr).String()).Error; err != nil {
		return nil, err
	}
	return &types.RetrievalAsk{
		Miner:                   addr,
		PricePerByte:            fbig.Int{Int: mAsk.PricePerByte.Int},
		UnsealPrice:             fbig.Int{Int: mAsk.UnsealPrice.Int},
		PaymentInterval:         mAsk.PaymentInterval,
		PaymentIntervalIncrease: mAsk.PaymentIntervalIncrease,
	}, nil
}

func (r *retrievalAskRepo) SetAsk(ask *types.RetrievalAsk) error {
	return r.ds.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		UpdateAll: true,
	}).Save(&modelRetrievalAsk{
		Address:                 DBAddress(ask.Miner),
		PricePerByte:            convertBigInt(ask.PricePerByte),
		UnsealPrice:             convertBigInt(ask.UnsealPrice),
		PaymentInterval:         ask.PaymentInterval,
		PaymentIntervalIncrease: ask.PaymentIntervalIncrease,
		TimeStampOrm:            TimeStampOrm{UpdatedAt: uint64(time.Now().Unix())},
	}).Error
}
