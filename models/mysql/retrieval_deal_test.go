package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-messager/models/mtypes"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
)

var dbDealCase *retrievalDeal
var dealStateCase *types.ProviderDealState

func init() {
	peerId, err := getTestPeerId()
	if err != nil {
		panic(err)
	}

	dbDealCase = &retrievalDeal{
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

	dealStateCase, err = toProviderDealState(dbDealCase)
	if err != nil {
		panic(err)
	}
	dealStateCase.ChannelID = &datatransfer.ChannelID{
		ID: datatransfer.TransferID(dbDealCase.ChannelID.ID),
	}
}

func TestRetrievalDealRepo(t *testing.T) {
	r, mock, sqlDB := setup(t)
	t.Run("mysql test SaveDeal", wrapper(testSaveDeal, r, mock))
	t.Run("mysql test GetDeal", wrapper(testGetDeal, r, mock))
	t.Run("mysql test GetDealByTransferId", wrapper(testGetDealByTransferId, r, mock))
	t.Run("mysql test HasDeal", wrapper(testHasDeal, r, mock))
	t.Run("mysql test ListDeals", wrapper(testListDeals, r, mock))
	t.Run("mysql test GroupRetrievalDealNumberByStatus", wrapper(testGroupRetrievalDealNumberByStatus, r, mock))

	assert.NoError(t, closeDB(mock, sqlDB))
}

func testSaveDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `retrieval_deals` (`cdp_payload_cid`,`cdp_selector`,`cdp_piece_cid`,`cdp_price_perbyte`,`cdp_payment_interval`,`cdp_payment_interval_increase`,`cdp_unseal_price`,`store_id`,`ci_initiator`,`ci_responder`,`ci_channel_id`,`sel_proposal_cid`,`status`,`receiver`,`total_sent`,`funds_received`,`message`,`current_interval`,`legacy_protocol`,`created_at`,`updated_at`,`cdp_proposal_id`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE `cdp_payload_cid`=VALUES(`cdp_payload_cid`),`cdp_selector`=VALUES(`cdp_selector`),`cdp_piece_cid`=VALUES(`cdp_piece_cid`),`cdp_price_perbyte`=VALUES(`cdp_price_perbyte`),`cdp_payment_interval`=VALUES(`cdp_payment_interval`),`cdp_payment_interval_increase`=VALUES(`cdp_payment_interval_increase`),`cdp_unseal_price`=VALUES(`cdp_unseal_price`),`store_id`=VALUES(`store_id`),`ci_initiator`=VALUES(`ci_initiator`),`ci_responder`=VALUES(`ci_responder`),`ci_channel_id`=VALUES(`ci_channel_id`),`sel_proposal_cid`=VALUES(`sel_proposal_cid`),`status`=VALUES(`status`),`total_sent`=VALUES(`total_sent`),`funds_received`=VALUES(`funds_received`),`message`=VALUES(`message`),`current_interval`=VALUES(`current_interval`),`legacy_protocol`=VALUES(`legacy_protocol`),`updated_at`=VALUES(`updated_at`)")).WithArgs(dbDealCase.DealProposal.PayloadCID, dbDealCase.DealProposal.Selector, dbDealCase.DealProposal.PieceCID, dbDealCase.DealProposal.PricePerByte, dbDealCase.DealProposal.PaymentInterval, dbDealCase.DealProposal.PaymentIntervalIncrease, dbDealCase.DealProposal.UnsealPrice, dbDealCase.StoreID, dbDealCase.ChannelID.Initiator, dbDealCase.ChannelID.Responder, dbDealCase.ChannelID.ID, dbDealCase.SelStorageProposalCid, dbDealCase.Status, dbDealCase.Receiver, dbDealCase.TotalSent, dbDealCase.FundsReceived, dbDealCase.Message, dbDealCase.CurrentInterval, dbDealCase.LegacyProtocol, sqlmock.AnyArg(), sqlmock.AnyArg(), dbDealCase.DealProposal.ID).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.RetrievalDealRepo().SaveDeal(ctx, dealStateCase)
	assert.Nil(t, err)
}

func testGetDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	peerid, err := peer.Decode(dbDealCase.Receiver)
	assert.Nil(t, err)

	rows := mock.NewRows([]string{"cdp_proposal_id", "cdp_payload_cid", "cdp_selector", "cdp_piece_cid", "cdp_price_perbyte", "cdp_payment_interval", "cdp_payment_interval_increase", "cdp_unseal_price", "store_id", "ci_initiator", "ci_responder", "ci_channel_id", "sel_proposal_cid", "status", "receiver", "total_sent", "funds_received", "message", "current_interval", "legacy_protocol", "created_at", "updated_at"}).AddRow(dbDealCase.ID, []byte(dbDealCase.DealProposal.PayloadCID.String()), dbDealCase.DealProposal.Selector, []byte(dbDealCase.DealProposal.PieceCID.String()), dbDealCase.DealProposal.PricePerByte, dbDealCase.DealProposal.PaymentInterval, dbDealCase.DealProposal.PaymentIntervalIncrease, dbDealCase.DealProposal.UnsealPrice, dbDealCase.StoreID, dbDealCase.ChannelID.Initiator, dbDealCase.ChannelID.Responder, dbDealCase.ChannelID.ID, []byte(dbDealCase.SelStorageProposalCid.String()), dbDealCase.Status, dbDealCase.Receiver, dbDealCase.TotalSent, dbDealCase.FundsReceived, dbDealCase.Message, dbDealCase.CurrentInterval, dbDealCase.LegacyProtocol, dbDealCase.CreatedAt, dbDealCase.UpdatedAt)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE cdp_proposal_id=? AND receiver=? LIMIT 1")).WithArgs(retrievalmarket.DealID(dbDealCase.DealProposal.ID), peerid.String()).WillReturnRows(rows)

	res, err := r.RetrievalDealRepo().GetDeal(ctx, peerid, retrievalmarket.DealID(dbDealCase.DealProposal.ID))
	assert.Nil(t, err)
	dealState, err := toProviderDealState(dbDealCase)
	assert.Equal(t, res, dealState)
}

func testGetDealByTransferId(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	rows := mock.NewRows([]string{"cdp_proposal_id", "cdp_payload_cid", "cdp_selector", "cdp_piece_cid", "cdp_price_perbyte", "cdp_payment_interval", "cdp_payment_interval_increase", "cdp_unseal_price", "store_id", "ci_initiator", "ci_responder", "ci_channel_id", "sel_proposal_cid", "status", "receiver", "total_sent", "funds_received", "message", "current_interval", "legacy_protocol", "created_at", "updated_at"}).AddRow(dbDealCase.ID, []byte(dbDealCase.DealProposal.PayloadCID.String()), dbDealCase.DealProposal.Selector, []byte(dbDealCase.DealProposal.PieceCID.String()), dbDealCase.DealProposal.PricePerByte, dbDealCase.DealProposal.PaymentInterval, dbDealCase.DealProposal.PaymentIntervalIncrease, dbDealCase.DealProposal.UnsealPrice, dbDealCase.StoreID, dbDealCase.ChannelID.Initiator, dbDealCase.ChannelID.Responder, dbDealCase.ChannelID.ID, []byte(dbDealCase.SelStorageProposalCid.String()), dbDealCase.Status, dbDealCase.Receiver, dbDealCase.TotalSent, dbDealCase.FundsReceived, dbDealCase.Message, dbDealCase.CurrentInterval, dbDealCase.LegacyProtocol, dbDealCase.CreatedAt, dbDealCase.UpdatedAt)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` WHERE ci_initiator = ? AND ci_responder = ? AND ci_channel_id = ? LIMIT 1")).WithArgs(dbDealCase.ChannelID.Initiator, dbDealCase.ChannelID.Responder, dbDealCase.ChannelID.ID).WillReturnRows(rows)

	res, err := r.RetrievalDealRepo().GetDealByTransferId(ctx, datatransfer.ChannelID{
		ID: datatransfer.TransferID(dbDealCase.ChannelID.ID),
	})
	assert.Nil(t, err)
	dealState, err := toProviderDealState(dbDealCase)
	assert.Equal(t, res, dealState)
}

func testHasDeal(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
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

func testListDeals(t *testing.T, r repo.Repo, mock sqlmock.Sqlmock) {
	ctx := context.Background()

	rows := mock.NewRows([]string{"cdp_proposal_id", "cdp_payload_cid", "cdp_selector", "cdp_piece_cid", "cdp_price_perbyte", "cdp_payment_interval", "cdp_payment_interval_increase", "cdp_unseal_price", "store_id", "ci_initiator", "ci_responder", "ci_channel_id", "sel_proposal_cid", "status", "receiver", "total_sent", "funds_received", "message", "current_interval", "legacy_protocol", "created_at", "updated_at"})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` LIMIT 10 OFFSET 10")).WillReturnRows(rows)
	res, err := r.RetrievalDealRepo().ListDeals(ctx, 2, 10)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(res))

	rows = mock.NewRows([]string{"cdp_proposal_id", "cdp_payload_cid", "cdp_selector", "cdp_piece_cid", "cdp_price_perbyte", "cdp_payment_interval", "cdp_payment_interval_increase", "cdp_unseal_price", "store_id", "ci_initiator", "ci_responder", "ci_channel_id", "sel_proposal_cid", "status", "receiver", "total_sent", "funds_received", "message", "current_interval", "legacy_protocol", "created_at", "updated_at"}).AddRow(dbDealCase.ID, []byte(dbDealCase.DealProposal.PayloadCID.String()), dbDealCase.DealProposal.Selector, []byte(dbDealCase.DealProposal.PieceCID.String()), dbDealCase.DealProposal.PricePerByte, dbDealCase.DealProposal.PaymentInterval, dbDealCase.DealProposal.PaymentIntervalIncrease, dbDealCase.DealProposal.UnsealPrice, dbDealCase.StoreID, dbDealCase.ChannelID.Initiator, dbDealCase.ChannelID.Responder, dbDealCase.ChannelID.ID, []byte(dbDealCase.SelStorageProposalCid.String()), dbDealCase.Status, dbDealCase.Receiver, dbDealCase.TotalSent, dbDealCase.FundsReceived, dbDealCase.Message, dbDealCase.CurrentInterval, dbDealCase.LegacyProtocol, dbDealCase.CreatedAt, dbDealCase.UpdatedAt)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `retrieval_deals` LIMIT 10")).WillReturnRows(rows)
	res2, err := r.RetrievalDealRepo().ListDeals(ctx, 1, 10)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res2))
	dealState, err := toProviderDealState(dbDealCase)
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

	addr, err := address.NewIDAddress(10)
	assert.Nil(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT state, count(1) as count FROM `retrieval_deals` GROUP BY `state`")).WillReturnRows(rows)
	result, err := r.RetrievalDealRepo().GroupRetrievalDealNumberByStatus(ctx, addr)
	assert.Nil(t, err)
	assert.Equal(t, expectResult, result)
}
