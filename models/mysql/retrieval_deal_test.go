package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-messager/models/mtypes"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

var dbRetrievalDealCase *retrievalDeal
var RetrievaldealStateCase *types.ProviderDealState

func TestRetrievalDealRepo(t *testing.T) {
	peerId, err := getTestPeerId()
	assert.NoError(t, err)

	dbRetrievalDealCase = &retrievalDeal{
		DealProposal: DealProposal{
			ID:           1,
			PricePerByte: mtypes.NewInt(1),
			UnsealPrice:  mtypes.NewInt(1),
		},
		Receiver:      peerId.String(),
		FundsReceived: mtypes.NewInt(1),
		StoreID:       1,
		ChannelID: ChannelID{
			ID: 0,
		},
		Message:         "test-message",
		CurrentInterval: 1,
		LegacyProtocol:  true,
		TimeStampOrm:    TimeStampOrm{CreatedAt: uint64(time.Now().Unix()), UpdatedAt: uint64(time.Now().Unix())},
	}

	RetrievaldealStateCase, err = toProviderDealState(dbRetrievalDealCase)
	assert.NoError(t, err)
	RetrievaldealStateCase.ChannelID = &datatransfer.ChannelID{
		ID: datatransfer.TransferID(dbRetrievalDealCase.ChannelID.ID),
	}

	r, mock, sqlDB := setup(t)
	t.Run("mysql test SaveDeal", wrapper(testSaveRetrievalDeal, r, mock))
	t.Run("mysql test GetDeal", wrapper(testRetrievalGetDeal, r, mock))
	t.Run("mysql test GetDealByTransferId", wrapper(testGetRetrievalDealByTransferId, r, mock))
	t.Run("mysql test HasDeal", wrapper(testHasRetrievalDeal, r, mock))
	t.Run("mysql test ListDeals", wrapper(testListRetrievalDeals, r, mock))
	t.Run("mysql test GroupRetrievalDealNumberByStatus", wrapper(testGroupRetrievalDealNumberByStatus, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func testSaveRetrievalDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()
	dbDeal := fromProviderDealState(RetrievaldealStateCase)

	db, err := getMysqlDryrunDB()
	assert.Nil(t, err)
	sql, vars, err := getSQL(db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).
		Create(dbDeal))
	assert.NoError(t, err)
	vars[20] = sqlmock.AnyArg()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(sql)).WithArgs(vars...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.RetrievalDealRepo().SaveDeal(ctx, RetrievaldealStateCase)
	assert.Nil(t, err)
}

func testRetrievalGetDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	peerid, err := peer.Decode(dbRetrievalDealCase.Receiver)
	assert.Nil(t, err)

	rows, err := getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE cdp_proposal_id=? AND receiver=? LIMIT 1")).WithArgs(retrievalmarket.DealID(dbRetrievalDealCase.DealProposal.ID), peerid.String()).WillReturnRows(rows)

	res, err := r.RetrievalDealRepo().GetDeal(ctx, peerid, retrievalmarket.DealID(dbRetrievalDealCase.DealProposal.ID))
	assert.Nil(t, err)
	dealState, err := toProviderDealState(dbRetrievalDealCase)
	assert.NoError(t, err)
	assert.Equal(t, res, dealState)
}

func testGetRetrievalDealByTransferId(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	rows, err := getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE ci_initiator = ? AND ci_responder = ? AND ci_channel_id = ? LIMIT 1")).WithArgs(dbRetrievalDealCase.ChannelID.Initiator, dbRetrievalDealCase.ChannelID.Responder, dbRetrievalDealCase.ChannelID.ID).WillReturnRows(rows)

	res, err := r.RetrievalDealRepo().GetDealByTransferId(ctx, datatransfer.ChannelID{
		ID: datatransfer.TransferID(dbRetrievalDealCase.ChannelID.ID),
	})
	assert.Nil(t, err)
	dealState, err := toProviderDealState(dbRetrievalDealCase)
	assert.NoError(t, err)
	assert.Equal(t, res, dealState)
}

func testHasRetrievalDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()
	did := retrievalmarket.DealID(1)
	peerId, err := getTestPeerId()
	assert.Nil(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `retrieval_deals` WHERE cdp_proposal_id=? AND receiver=? ")).WithArgs(did, peerId.String()).WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
	has, err := r.RetrievalDealRepo().HasDeal(ctx, peerId, did)
	assert.Nil(t, err)
	assert.False(t, has)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `retrieval_deals` WHERE cdp_proposal_id=? AND receiver=? ")).WithArgs(did, peerId.String()).WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	has, err = r.RetrievalDealRepo().HasDeal(ctx, peerId, did)
	assert.Nil(t, err)
	assert.True(t, has)
}

func testListRetrievalDeals(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	rows := mock.NewRows([]string{"cdp_proposal_id", "cdp_payload_cid", "cdp_selector", "cdp_piece_cid", "cdp_price_perbyte", "cdp_payment_interval", "cdp_payment_interval_increase", "cdp_unseal_price", "store_id", "ci_initiator", "ci_responder", "ci_channel_id", "sel_proposal_cid", "status", "receiver", "total_sent", "funds_received", "message", "current_interval", "legacy_protocol", "created_at", "updated_at"})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` LIMIT 10 OFFSET 10")).WillReturnRows(rows)
	res, err := r.RetrievalDealRepo().ListDeals(ctx, 2, 10)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(res))

	rows, err = getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` LIMIT 10")).WillReturnRows(rows)
	res2, err := r.RetrievalDealRepo().ListDeals(ctx, 1, 10)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res2))
	dealState, err := toProviderDealState(dbRetrievalDealCase)
	assert.NoError(t, err)
	assert.Equal(t, res2[0], dealState)
}

func testGroupRetrievalDealNumberByStatus(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()
	expectResult := map[retrievalmarket.DealStatus]int64{
		retrievalmarket.DealStatusAccepted:       1,
		retrievalmarket.DealStatusErrored:        1432,
		retrievalmarket.DealStatusCompleted:      13,
		retrievalmarket.DealStatusBlocksComplete: 100,
	}
	rows := mock.NewRows([]string{"state", "count"})
	for status, count := range expectResult {
		rows.AddRow(status, count)
	}

	addr := getTestAddress()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT state, count(1) as count FROM `retrieval_deals` GROUP BY `state`")).WillReturnRows(rows)
	result, err := r.RetrievalDealRepo().GroupRetrievalDealNumberByStatus(ctx, addr)
	assert.Nil(t, err)
	assert.Equal(t, expectResult, result)
}
