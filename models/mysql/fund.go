package mysql

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/types"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"gorm.io/gorm"
)

type fundedAddressState struct {
	Addr        Address    `gorm:"column:addr;type:varchar(128);primary_key"`
	AmtReserved mtypes.Int `gorm:"column:amt_reserved;type:varchar(256);"`
	MsgCid      string     `gorm:"column:msg_cid;type:varchar(128);"`
}

func (fas *fundedAddressState) TableName() string {
	return "funded_address_state"
}

func fromFundedAddressState(src *types.FundedAddressState) *fundedAddressState {
	fds := &fundedAddressState{
		Addr:        toAddress(src.Addr),
		MsgCid:      decodeCidPtr(src.MsgCid),
		AmtReserved: convertBigInt(src.AmtReserved),
	}

	return fds
}

func toFundedAddressState(src *fundedAddressState) (*types.FundedAddressState, error) {
	fds := &types.FundedAddressState{
		AmtReserved: abi.TokenAmount{Int: src.AmtReserved.Int},
		Addr:        src.Addr.addr(),
	}

	var err error
	fds.MsgCid, err = parseCidPtr(src.MsgCid)
	if err != nil {
		return nil, err
	}

	return fds, nil
}

type fundedAddressStateRepo struct {
	*gorm.DB
}

func NewFundedAddressStateRepo(db *gorm.DB) *fundedAddressStateRepo {
	return &fundedAddressStateRepo{db}
}

func (f *fundedAddressStateRepo) SaveFundedAddressState(fds *types.FundedAddressState) error {
	return f.DB.Save(fromFundedAddressState(fds)).Error
}

func (f *fundedAddressStateRepo) GetFundedAddressState(addr address.Address) (*types.FundedAddressState, error) {
	var fas fundedAddressState
	err := f.DB.Take(&fas, "addr = ?", cutPrefix(addr)).Error
	if err != nil {
		return nil, err
	}

	return toFundedAddressState(&fas)
}

func (f *fundedAddressStateRepo) ListFundedAddressState() ([]*types.FundedAddressState, error) {
	var fads []*fundedAddressState
	err := f.DB.Find(&fads).Error
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
