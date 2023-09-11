package mysql

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"gorm.io/gorm"
)

const directDealTableName = "direct_deals"

type directDeal struct {
	ID        string    `gorm:"column:id;type:varchar(128);primary_key"`
	PieceCID  DBCid     `gorm:"column:piece_cid;type:varchar(256);index"`
	PieceSize uint64    `gorm:"column:piece_size;type:bigint unsigned;NOT NULL"`
	Client    DBAddress `gorm:"column:client;type:varchar(256);index"`
	Provider  DBAddress `gorm:"column:provider;type:varchar(256);index"`

	State   types.DirectDealState `gorm:"column:state;type:int;NOT NULL"`
	Status  types.PieceStatus     `gorm:"column:status;type:varchar(32);NOT NULL"`
	Message string                `gorm:"column:message;type:varchar(256)"`

	AllocationID uint64 `gorm:"column:allocation_id;type:bigint unsigned;index;NOT NULL"`
	ClaimID      uint64 `gorm:"column:claim_id;type:bigint unsigned;NOT NULL"`

	SectorID uint64 `gorm:"column:sector_id;type:bigint unsigned;NOT NULL"`
	Offset   uint64 `gorm:"column:offset;type:bigint unsigned;NOT NULL"`
	Length   uint64 `gorm:"column:length;type:bigint unsigned;NOT NULL"`

	StartEpoch int64 `gorm:"column:start_epoch;type:bigint;NOT NULL"`
	EndEpoch   int64 `gorm:"column:end_epoch;type:bigint;NOT NULL"`

	TimeStampOrm
}

func (dd *directDeal) TableName() string {
	return directDealTableName
}

func (dd *directDeal) toDirectDeal() (*types.DirectDeal, error) {
	deal := &types.DirectDeal{
		PieceCID:     dd.PieceCID.cid(),
		PieceSize:    abi.PaddedPieceSize(dd.PieceSize),
		Client:       dd.Client.addr(),
		Provider:     dd.Provider.addr(),
		State:        dd.State,
		PieceStatus:  dd.Status,
		Message:      dd.Message,
		AllocationID: verifreg.AllocationId(dd.AllocationID),
		ClaimID:      verifreg.ClaimId(dd.ClaimID),
		SectorID:     abi.SectorNumber(dd.SectorID),
		Length:       abi.PaddedPieceSize(dd.Length),
		Offset:       abi.PaddedPieceSize(dd.Offset),
		StartEpoch:   abi.ChainEpoch(dd.StartEpoch),
		EndEpoch:     abi.ChainEpoch(dd.EndEpoch),
		TimeStamp:    dd.Timestamp(),
	}
	id, err := uuid.Parse(dd.ID)
	if err != nil {
		return nil, err
	}
	deal.ID = id

	return deal, nil
}

func fromDirectDeal(dd *types.DirectDeal) *directDeal {
	return &directDeal{
		ID:           dd.ID.String(),
		PieceCID:     DBCid(dd.PieceCID),
		PieceSize:    uint64(dd.PieceSize),
		Client:       DBAddress(dd.Client),
		Provider:     DBAddress(dd.Provider),
		State:        dd.State,
		Status:       dd.PieceStatus,
		Message:      dd.Message,
		AllocationID: uint64(dd.AllocationID),
		ClaimID:      uint64(dd.ClaimID),
		SectorID:     uint64(dd.SectorID),
		Length:       uint64(dd.Length),
		Offset:       uint64(dd.Offset),
		StartEpoch:   int64(dd.StartEpoch),
		EndEpoch:     int64(dd.EndEpoch),
		TimeStampOrm: TimeStampOrm{
			CreatedAt: dd.CreatedAt,
			UpdatedAt: dd.CreatedAt,
		},
	}
}

type directDealRepo struct {
	*gorm.DB
}

func NewDirectDealRepo(db *gorm.DB) repo.DirectDealRepo {
	return &directDealRepo{DB: db}
}

func (ddr *directDealRepo) SaveDeal(ctx context.Context, deal *types.DirectDeal) error {
	d := fromDirectDeal(deal)
	d.TimeStampOrm.Refresh()

	return ddr.DB.WithContext(ctx).Save(d).Error
}

func (ddr *directDealRepo) GetDeal(ctx context.Context, id uuid.UUID) (*types.DirectDeal, error) {
	var deal directDeal
	if err := ddr.DB.WithContext(ctx).Take(&deal, "id = ?", id.String()).Error; err != nil {
		return nil, err
	}

	return deal.toDirectDeal()
}

func (ddr *directDealRepo) GetDealByAllocationID(ctx context.Context, allocationID uint64) (*types.DirectDeal, error) {
	var deal directDeal
	if err := ddr.DB.WithContext(ctx).Take(&deal, "allocation_id = ? and state != ?", allocationID, types.DealError).Error; err != nil {
		return nil, err
	}

	return deal.toDirectDeal()
}

func (ddr *directDealRepo) ListDeal(ctx context.Context) ([]*types.DirectDeal, error) {
	var deals []*directDeal
	if err := ddr.DB.WithContext(ctx).Find(&deals).Error; err != nil {
		return nil, err
	}

	out := make([]*types.DirectDeal, 0, len(deals))
	for _, deal := range deals {
		d, err := deal.toDirectDeal()
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}

	return out, nil
}
