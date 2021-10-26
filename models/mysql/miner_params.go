package mysql

import (
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/types"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"gorm.io/gorm"
)

type minerParams struct {
	Miner         string         `gorm:"column:miner;type:varchar(256);primary_key"`
	Price         mtypes.Int     `gorm:"column:price;type:varchar(256);"`
	VerifiedPrice mtypes.Int     `gorm:"column:verified_price;type:varchar(256);"`
	Duration      abi.ChainEpoch `gorm:"column:duration;type:bigint;"`
	MinPieceSize  int64          `gorm:"column:min_piece_size;type:bigint;"`
	MaxPieceSize  int64          `gorm:"column:max_piece_size;type:bigint;"`

	SignerToken string `gorm:"column:signer_token;type:varchar(256);"`

	CreatedAt time.Time `gorm:"column:created_at;"`
	UpdateAt  time.Time `gorm:"column:updated_at;"`
}

func fromMinerParams(src *types.MinerParams) *minerParams {
	params := &minerParams{
		Miner:        src.Miner.String(),
		Duration:     src.Duration,
		MinPieceSize: src.MinPieceSize,
		MaxPieceSize: src.MaxPieceSize,
		SignerToken:  src.SignerToken,
		CreatedAt:    src.CreatedAt,
		UpdateAt:     src.UpdateAt,
	}
	if !src.Price.Nil() {
		params.Price = mtypes.NewFromGo(src.Price.Int)
	} else {
		params.Price = mtypes.Zero()
	}

	if !src.VerifiedPrice.Nil() {
		params.VerifiedPrice = mtypes.NewFromGo(src.VerifiedPrice.Int)
	} else {
		params.VerifiedPrice = mtypes.Zero()
	}

	return params
}

func toMinerParams(src *minerParams) (*types.MinerParams, error) {
	params := &types.MinerParams{
		Price:         src.Price,
		VerifiedPrice: src.VerifiedPrice,
		Duration:      src.Duration,
		MinPieceSize:  src.MinPieceSize,
		MaxPieceSize:  src.MaxPieceSize,
		SignerToken:   src.SignerToken,
		CreatedAt:     src.CreatedAt,
		UpdateAt:      src.UpdateAt,
	}
	addr, err := address.NewFromString(src.Miner)
	if err != nil {
		return nil, err
	}
	params.Miner = addr

	return params, nil
}

func (mp *minerParams) TableName() string {
	return "miner_params"
}

type minerParamsRepo struct {
	*gorm.DB
}

func NewMinerParamsRepo(db *gorm.DB) *minerParamsRepo {
	return &minerParamsRepo{db}
}

func (m *minerParamsRepo) CreateMinerParams(params *types.MinerParams) error {
	params.CreatedAt = time.Now()
	params.UpdateAt = time.Now()
	return m.DB.Create(fromMinerParams(params)).Error
}

func (m *minerParamsRepo) GetMinerParams(miner address.Address) (*types.MinerParams, error) {
	var params minerParams
	err := m.DB.Take(&params, "miner = ?", miner.String()).Error
	if err != nil {
		return nil, err
	}

	return toMinerParams(&params)
}

func (m *minerParamsRepo) UpdateMinerParams(miner address.Address, updateCols map[string]interface{}) error {
	return m.DB.Model(&minerParams{}).UpdateColumns(updateCols).Error
}

func (m *minerParamsRepo) ListMinerParams() ([]*types.MinerParams, error) {
	var list []*minerParams
	err := m.DB.Find(&list).Error
	if err != nil {
		return nil, err
	}
	minerParams := make([]*types.MinerParams, 0, len(list))
	for _, one := range list {
		tmp, err := toMinerParams(one)
		if err != nil {
			return nil, err
		}
		minerParams = append(minerParams, tmp)
	}

	return minerParams, nil
}

//var _ repo.MinerParamsRepo = (*minerParamsRepo)(nil)
