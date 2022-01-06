package mysql

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/types"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"gorm.io/gorm"
)

const fundedAddressStateTableName = "funded_address_state"

type fundedAddressState struct {
	Addr        DBAddress  `gorm:"column:addr;type:varchar(256);primary_key"`
	AmtReserved mtypes.Int `gorm:"column:amt_reserved;type:varchar(256);"`
	MsgCid      DBCid      `gorm:"column:msg_cid;type:varchar(256);"`
	TimeStampOrm
}

func (fas *fundedAddressState) TableName() string {
	return fundedAddressStateTableName
}

func fromFundedAddressState(src *types.FundedAddressState) *fundedAddressState {
	fds := &fundedAddressState{
		Addr:        DBAddress(src.Addr),
		AmtReserved: convertBigInt(src.AmtReserved),
	}
	if src.MsgCid == nil {
		fds.MsgCid = UndefDBCid
	} else {
		fds.MsgCid = DBCid(*src.MsgCid)
	}

	return fds
}

func toFundedAddressState(src *fundedAddressState) (*types.FundedAddressState, error) {
	fds := &types.FundedAddressState{
		AmtReserved: abi.TokenAmount{Int: src.AmtReserved.Int},
		MsgCid:      src.MsgCid.cidPtr(),
		Addr:        src.Addr.addr(),
	}

	return fds, nil
}

type fundedAddressStateRepo struct {
	*gorm.DB
}

func NewFundedAddressStateRepo(db *gorm.DB) *fundedAddressStateRepo {
	return &fundedAddressStateRepo{db}
}

func (far *fundedAddressStateRepo) SaveFundedAddressState(ctx context.Context, fds *types.FundedAddressState) error {
	state := fromFundedAddressState(fds)
	state.UpdatedAt = uint64(time.Now().Unix())
	return far.WithContext(ctx).Save(state).Error
}

func (far *fundedAddressStateRepo) GetFundedAddressState(ctx context.Context, addr address.Address) (*types.FundedAddressState, error) {
	var fas fundedAddressState
	err := far.WithContext(ctx).Take(&fas, "addr = ?", DBAddress(addr).String()).Error
	if err != nil {
		return nil, err
	}

	return toFundedAddressState(&fas)
}

func (far *fundedAddressStateRepo) ListFundedAddressState(ctx context.Context) ([]*types.FundedAddressState, error) {
	var fads []*fundedAddressState
	err := far.WithContext(ctx).Find(&fads).Error
	if err != nil {
		return nil, err
	}
	list := make([]*types.FundedAddressState, 0, len(fads))
	for _, fad := range fads {
		newFad, err := toFundedAddressState(fad)
		if err != nil {
			return nil, err
		}
		list = append(list, newFad)
	}

	return list, nil
}
