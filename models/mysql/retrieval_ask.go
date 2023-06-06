package mysql

import (
	"context"

	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-messager/models/mtypes"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const retrievalAskTableName = "retrieval_asks"

type retrievalAskRepo struct {
	*gorm.DB
}

var _ repo.IRetrievalAskRepo = (*retrievalAskRepo)(nil)

func NewRetrievalAskRepo(db *gorm.DB) repo.IRetrievalAskRepo {
	return &retrievalAskRepo{db}
}

type retrievalAsk struct {
	ID                      uint       `gorm:"primary_key"`
	Address                 DBAddress  `gorm:"column:address;uniqueIndex;type:varchar(256)"`
	PricePerByte            mtypes.Int `gorm:"column:price_per_byte;type:varchar(256);default:0"`
	UnsealPrice             mtypes.Int `gorm:"column:unseal_price;type:varchar(256);default:0"`
	PaymentInterval         uint64     `gorm:"column:payment_interval;type:bigint unsigned;NOT NULL;"`
	PaymentIntervalIncrease uint64     `gorm:"column:payment_interval_increase;type:bigint unsigned;NOT NULL;"`
	TimeStampOrm
}

func (a *retrievalAsk) TableName() string {
	return retrievalAskTableName
}

func (rar *retrievalAskRepo) GetAsk(ctx context.Context, addr address.Address) (*types.RetrievalAsk, error) {
	var mAsk retrievalAsk
	if err := rar.WithContext(ctx).Take(&mAsk, "address = ?", DBAddress(addr).String()).Error; err != nil {
		return nil, err
	}
	return &types.RetrievalAsk{
		Miner:                   addr,
		PricePerByte:            big.Int(mtypes.SafeFromGo(mAsk.PricePerByte.Int)),
		UnsealPrice:             big.Int(mtypes.SafeFromGo(mAsk.UnsealPrice.Int)),
		PaymentInterval:         mAsk.PaymentInterval,
		PaymentIntervalIncrease: mAsk.PaymentIntervalIncrease,
		TimeStamp:               mAsk.Timestamp(),
	}, nil
}

func (rar *retrievalAskRepo) SetAsk(ctx context.Context, ask *types.RetrievalAsk) error {
	return rar.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		UpdateAll: true,
	}).Create(&retrievalAsk{
		Address:                 DBAddress(ask.Miner),
		PricePerByte:            mtypes.SafeFromGo(ask.PricePerByte.Int),
		UnsealPrice:             mtypes.SafeFromGo(ask.UnsealPrice.Int),
		PaymentInterval:         ask.PaymentInterval,
		PaymentIntervalIncrease: ask.PaymentIntervalIncrease,
		TimeStampOrm:            *(&TimeStampOrm{CreatedAt: ask.CreatedAt, UpdatedAt: ask.UpdatedAt}).Refresh(),
	}).Error
}

func (rar *retrievalAskRepo) ListAsk(ctx context.Context) ([]*types.RetrievalAsk, error) {
	var dbAsks []retrievalAsk
	err := rar.WithContext(ctx).Table("retrieval_asks").Find(&dbAsks).Error
	if err != nil {
		return nil, err
	}
	results := make([]*types.RetrievalAsk, len(dbAsks))
	for index, ask := range dbAsks {
		results[index] = &types.RetrievalAsk{
			Miner:                   ask.Address.addr(),
			PricePerByte:            big.Int(mtypes.SafeFromGo(ask.PricePerByte.Int)),
			UnsealPrice:             big.Int(mtypes.SafeFromGo(ask.UnsealPrice.Int)),
			PaymentInterval:         ask.PaymentInterval,
			PaymentIntervalIncrease: ask.PaymentIntervalIncrease,
			TimeStamp:               ask.Timestamp(),
		}
	}
	return results, nil
}
