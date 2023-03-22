package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-messager/models/mtypes"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

func prepareRetrievalDealRepoTest(t *testing.T) (repo.Repo, sqlmock.Sqlmock, *retrievalDeal, *types.ProviderDealState, func()) {
	peerId, err := getTestPeerId()
	assert.NoError(t, err)

	dbRetrievalDealCase := &retrievalDeal{
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

	RetrievaldealStateCase, err := toProviderDealState(dbRetrievalDealCase)
	assert.NoError(t, err)
	RetrievaldealStateCase.ChannelID = &datatransfer.ChannelID{
		ID: datatransfer.TransferID(dbRetrievalDealCase.ChannelID.ID),
	}

	r, mock, sqlDB := setup(t)

	return r, mock, dbRetrievalDealCase, RetrievaldealStateCase, func() {
		assert.NoError(t, closeDB(mock, sqlDB))
	}
}

func TestSaveRetrievalDeal(t *testing.T) {
	r, mock, _, RetrievaldealStateCase, close := prepareRetrievalDealRepoTest(t)
	defer close()
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

func TestRetrievalGetDeal(t *testing.T) {
	r, mock, dbRetrievalDealCase, _, close := prepareRetrievalDealRepoTest(t)
	defer close()

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

func TestGetRetrievalDealByTransferId(t *testing.T) {
	r, mock, dbRetrievalDealCase, _, close := prepareRetrievalDealRepoTest(t)
	defer close()

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

func TestHasRetrievalDeal(t *testing.T) {
	r, mock, _, _, close := prepareRetrievalDealRepoTest(t)
	defer close()

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

func TestListRetrievalDeals(t *testing.T) {
	r, mock, dbRetrievalDealCase, _, close := prepareRetrievalDealRepoTest(t)
	defer close()

	ctx := context.Background()

	rows, err := getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals`")).WillReturnRows(rows)
	res, err := r.RetrievalDealRepo().ListDeals(ctx, &types.RetrievalDealQueryParams{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))

	rows = mock.NewRows([]string{"cdp_proposal_id", "cdp_payload_cid", "cdp_selector", "cdp_piece_cid", "cdp_price_perbyte", "cdp_payment_interval", "cdp_payment_interval_increase", "cdp_unseal_price", "store_id", "ci_initiator", "ci_responder", "ci_channel_id", "sel_proposal_cid", "status", "receiver", "total_sent", "funds_received", "message", "current_interval", "legacy_protocol", "created_at", "updated_at"})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` LIMIT 10 OFFSET 10")).WillReturnRows(rows)
	res, err = r.RetrievalDealRepo().ListDeals(ctx, &types.RetrievalDealQueryParams{Page: types.Page{Offset: 10, Limit: 10}})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(res))

	// test receiver
	receiver := dbRetrievalDealCase.Receiver
	rows, err = getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE receiver = ?")).WithArgs(receiver).WillReturnRows(rows)
	res, err = r.RetrievalDealRepo().ListDeals(ctx, &types.RetrievalDealQueryParams{Receiver: receiver})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))

	// test deal id
	dealID := dbRetrievalDealCase.ID
	rows, err = getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE cdp_proposal_id = ?")).WithArgs(dealID).WillReturnRows(rows)
	res, err = r.RetrievalDealRepo().ListDeals(ctx, &types.RetrievalDealQueryParams{DealID: abi.DealID(dealID)})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))

	// test status
	status := dbRetrievalDealCase.Status
	rows, err = getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE status = ?")).WithArgs(status).WillReturnRows(rows)
	res, err = r.RetrievalDealRepo().ListDeals(ctx, &types.RetrievalDealQueryParams{Status: &status})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))

	// test discard failed deal
	status = uint64(retrievalmarket.DealStatusErrored)
	rows, err = getFullRows(dbRetrievalDealCase)
	assert.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE status != ?")).WithArgs(status).WillReturnRows(rows)
	res, err = r.RetrievalDealRepo().ListDeals(ctx, &types.RetrievalDealQueryParams{DiscardFailedDeal: true})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))
}

func TestGroupRetrievalDealNumberByStatus(t *testing.T) {
	r, mock, _, _, close := prepareRetrievalDealRepoTest(t)
	defer close()

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
