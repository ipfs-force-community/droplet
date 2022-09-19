package mysql

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-messager/models/mtypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

const storageAskTableName = "storage_asks"

type storageAsk struct {
	ID            uint       `gorm:"primary_key"`
	Miner         DBAddress  `gorm:"column:miner;type:varchar(256);uniqueIndex"`
	Price         mtypes.Int `gorm:"column:price;type:varchar(256);"`
	VerifiedPrice mtypes.Int `gorm:"column:verified_price;type:varchar(256);"`
	MinPieceSize  int64      `gorm:"column:min_piece_size;type:bigint;"`
	MaxPieceSize  int64      `gorm:"column:max_piece_size;type:bigint;"`
	Timestamp     int64      `gorm:"column:timestamp;type:bigint;"`
	Expiry        int64      `gorm:"column:expiry;type:bigint;"`
	SeqNo         uint64     `gorm:"column:seq_no;type:bigint unsigned;"`
	Signature     Signature  `gorm:"column:signature;type:blob;"`
	TimeStampOrm
}

func (a *storageAsk) TableName() string {
	return storageAskTableName
}

func fromStorageAsk(src *types.SignedStorageAsk) *storageAsk {
	ask := &storageAsk{}
	if src.Ask != nil {
		ask.Miner = DBAddress(src.Ask.Miner)
		ask.Price = convertBigInt(src.Ask.Price)
		ask.VerifiedPrice = convertBigInt(src.Ask.VerifiedPrice)
		ask.MinPieceSize = int64(src.Ask.MinPieceSize)
		ask.MaxPieceSize = int64(src.Ask.MaxPieceSize)
		ask.Timestamp = int64(src.Ask.Timestamp)
		ask.Expiry = int64(src.Ask.Expiry)
		ask.SeqNo = src.Ask.SeqNo
	}
	if src.Signature != nil {
		ask.Signature = Signature{
			Type: src.Signature.Type,
			Data: src.Signature.Data,
		}
	}
	ask.TimeStampOrm = TimeStampOrm{CreatedAt: ask.CreatedAt, UpdatedAt: ask.UpdatedAt}
	return ask
}

func toStorageAsk(src *storageAsk) (*types.SignedStorageAsk, error) {
	ask := &types.SignedStorageAsk{
		Ask: &storagemarket.StorageAsk{
			Miner:         src.Miner.addr(),
			Price:         abi.TokenAmount{Int: src.Price.Int},
			VerifiedPrice: abi.TokenAmount{Int: src.VerifiedPrice.Int},
			MinPieceSize:  abi.PaddedPieceSize(src.MinPieceSize),
			MaxPieceSize:  abi.PaddedPieceSize(src.MaxPieceSize),
			Timestamp:     abi.ChainEpoch(src.Timestamp),
			Expiry:        abi.ChainEpoch(src.Expiry),
			SeqNo:         src.SeqNo,
		},
		TimeStamp: src.TimeStampOrm.Timestamp(),
	}
	if len(src.Signature.Data) != 0 {
		ask.Signature = &crypto.Signature{
			Type: src.Signature.Type,
			Data: src.Signature.Data,
		}
	}

	return ask, nil
}

type storageAskRepo struct {
	*gorm.DB
}

func NewStorageAskRepo(db *gorm.DB) repo.IStorageAskRepo {
	return &storageAskRepo{db}
}

func (sar *storageAskRepo) GetAsk(ctx context.Context, miner address.Address) (*types.SignedStorageAsk, error) {
	var res storageAsk
	err := sar.WithContext(ctx).Take(&res, "miner = ?", DBAddress(miner).String()).Error
	if err != nil {
		return nil, err
	}
	return toStorageAsk(&res)
}

func (sar *storageAskRepo) SetAsk(ctx context.Context, ask *types.SignedStorageAsk) error {
	if ask == nil || ask.Ask == nil {
		return fmt.Errorf("param is nil")
	}
	dbAsk := fromStorageAsk(ask)
	// I prefer setting `TimeStampOrm` to zero, letting `gorm` update automatically.
	dbAsk.TimeStampOrm = TimeStampOrm{}

	return sar.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "miner"}},
		UpdateAll: true,
	}).Save(dbAsk).Error
}

func (sar *storageAskRepo) ListAsk(ctx context.Context) ([]*types.SignedStorageAsk, error) {
	var dbAsks []storageAsk
	err := sar.Table("storage_asks").Find(&dbAsks).Error
	if err != nil {
		return nil, err
	}
	results := make([]*types.SignedStorageAsk, len(dbAsks))
	for index, ask := range dbAsks {
		mAsk, err := toStorageAsk(&ask)
		if err != nil {
			return nil, err
		}
		results[index] = mAsk
	}
	return results, nil
}
