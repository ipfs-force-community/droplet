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

var fundedAddressStatesCase = []*market_types.FundedAddressState{
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

var fundedAddressStateColumns = []string{"addr", "amt_reserved", "msg_cid", "created_at", "updated_at"}

func TestFundAddrState(t *testing.T) {
	r, mock, sqlDB := setup(t)

	t.Run("mysql test SaveFundedAddressState", wrapper(testSaveFundedAddressState, r, mock))
	t.Run("mysql test GetFundedAddressState", wrapper(testGetFundedAddressState, r, mock))
	t.Run("mysql test ListFundedAddressState", wrapper(testListFundedAddressState, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func testSaveFundedAddressState(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	fas := fromFundedAddressState(fundedAddressStatesCase[0])
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `funded_address_state` SET `amt_reserved`=?,`msg_cid`=?,`created_at`=?,`updated_at`=? WHERE `addr` = ?")).WithArgs(fas.AmtReserved, fas.MsgCid, fas.CreatedAt, sqlmock.AnyArg(), fas.Addr).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err := r.FundRepo().SaveFundedAddressState(ctx, fundedAddressStatesCase[0])
	assert.NoError(t, err)
}

func testGetFundedAddressState(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	fas := fromFundedAddressState(fundedAddressStatesCase[0])
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `funded_address_state` WHERE addr = ? LIMIT 1")).WithArgs(fas.Addr).WillReturnRows(sqlmock.NewRows(fundedAddressStateColumns).AddRow([]byte(fas.Addr.String()), fas.AmtReserved, []byte(fas.MsgCid.String()), fas.CreatedAt, fas.UpdatedAt))
	res, err := r.FundRepo().GetFundedAddressState(context.Background(), fundedAddressStatesCase[0].Addr)
	assert.NoError(t, err)
	assert.Equal(t, fundedAddressStatesCase[0], res)
}

func testListFundedAddressState(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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
