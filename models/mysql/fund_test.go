package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	market_types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/stretchr/testify/assert"
)

var fundedAddressStateColumns = []string{"addr", "amt_reserved", "msg_cid", "created_at", "updated_at"}

var prepareFundAddrStateTest = func(t *testing.T) (repo.Repo, sqlmock.Sqlmock, []*market_types.FundedAddressState, func()) {
	r, mock, sqlDB := setup(t)
	fundedAddressStatesCase := []*market_types.FundedAddressState{
		{
			Addr:        address.TestAddress,
			AmtReserved: abi.NewTokenAmount(100),
			MsgCid:      nil,
			TimeStamp:   market_types.TimeStamp{CreatedAt: uint64(time.Now().Unix()), UpdatedAt: uint64(time.Now().Unix())},
		},
		{
			Addr:        address.TestAddress2,
			AmtReserved: abi.NewTokenAmount(100),
			MsgCid:      nil,
			TimeStamp:   market_types.TimeStamp{CreatedAt: uint64(time.Now().Unix()), UpdatedAt: uint64(time.Now().Unix())},
		},
	}
	return r, mock, fundedAddressStatesCase, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func TestSaveFundedAddressState(t *testing.T) {
	r, mock, fundedAddressStatesCase, done := prepareFundAddrStateTest(t)
	defer done()

	ctx := context.Background()

	fas := fromFundedAddressState(fundedAddressStatesCase[0])
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `funded_address_state` SET `amt_reserved`=?,`msg_cid`=?,`created_at`=?,`updated_at`=? WHERE `addr` = ?")).WithArgs(fas.AmtReserved, fas.MsgCid, fas.CreatedAt, sqlmock.AnyArg(), fas.Addr).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err := r.FundRepo().SaveFundedAddressState(ctx, fundedAddressStatesCase[0])
	assert.NoError(t, err)
}

func TestGetFundedAddressState(t *testing.T) {
	r, mock, fundedAddressStatesCase, done := prepareFundAddrStateTest(t)
	defer done()

	fas := fromFundedAddressState(fundedAddressStatesCase[0])
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `funded_address_state` WHERE addr = ? LIMIT 1")).WithArgs(fas.Addr).WillReturnRows(sqlmock.NewRows(fundedAddressStateColumns).AddRow([]byte(fas.Addr.String()), fas.AmtReserved, []byte(fas.MsgCid.String()), fas.CreatedAt, fas.UpdatedAt))
	res, err := r.FundRepo().GetFundedAddressState(context.Background(), fundedAddressStatesCase[0].Addr)
	assert.NoError(t, err)
	assert.Equal(t, fundedAddressStatesCase[0], res)
}

func TestListFundedAddressState(t *testing.T) {
	r, mock, fundedAddressStatesCase, done := prepareFundAddrStateTest(t)
	defer done()

	rows := sqlmock.NewRows(fundedAddressStateColumns)
	for _, fas := range fundedAddressStatesCase {
		fas_ := fromFundedAddressState(fas)
		rows.AddRow([]byte(fas_.Addr.String()), fas_.AmtReserved, []byte(fas_.MsgCid.String()), fas_.CreatedAt, fas_.UpdatedAt)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `funded_address_state`")).WillReturnRows(rows)
	res, err := r.FundRepo().ListFundedAddressState(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, fundedAddressStatesCase[0], res[0])
	assert.Equal(t, fundedAddressStatesCase[1], res[1])
}
