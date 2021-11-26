package mysql

import (
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"golang.org/x/xerrors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func fromStorageAsk(src *storagemarket.SignedStorageAsk) *storageAsk {
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

	return ask
}

func toStorageAsk(src *storageAsk) (*storagemarket.SignedStorageAsk, error) {
	ask := &storagemarket.SignedStorageAsk{
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

func NewStorageAskRepo(db *gorm.DB) *storageAskRepo {
	return &storageAskRepo{db}
}

func (a *storageAskRepo) GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	var res storageAsk
	err := a.DB.Take(&res, "miner = ?", DBAddress(miner).String()).Error
	if err != nil {
		return nil, err
	}
	return toStorageAsk(&res)
}

func (a *storageAskRepo) SetAsk(ask *storagemarket.SignedStorageAsk) error {
	if ask == nil || ask.Ask == nil {
		return xerrors.Errorf("param is nil")
	}
	dbAsk := fromStorageAsk(ask)
	dbAsk.UpdatedAt = uint64(time.Now().Unix())
	return a.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "miner"}},
		UpdateAll: true,
	}).Save(dbAsk).Error
}

func (a *storageAskRepo) ListAsk() ([]*storagemarket.SignedStorageAsk, error) {
	var dbAsks []storageAsk
	err := a.Table("storage_asks").Find(&dbAsks).Error
	if err != nil {
		return nil, err
	}
	results := make([]*storagemarket.SignedStorageAsk, len(dbAsks))
	for index, ask := range dbAsks {
		mAsk, err := toStorageAsk(&ask)
		if err != nil {
			return nil, err
		}
		results[index] = mAsk
	}
	return results, nil
}
