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
	ID        string
	PieceCID  DBCid
	PieceSize uint64
	Client    DBAddress
	Provider  DBAddress

	AllocationID uint64
	ClaimID      uint64

	SectorID uint64
	Offset   uint64
	Length   uint64

	StartEpoch int64
	EndEpoch   int64

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
