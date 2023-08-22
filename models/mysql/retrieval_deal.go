package mysql

import (
	"bytes"
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/sophon-messager/models/mtypes"
	"github.com/libp2p/go-libp2p/core/peer"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const retrievalDealTableName = "retrieval_deals"

type retrievalDeal struct {
	DealProposal          `gorm:"embedded;embeddedPrefix:cdp_"`
	StoreID               uint64     `gorm:"column:store_id;type:bigint unsigned;NOT NULL;"`
	ChannelID             ChannelID  `gorm:"embedded;embeddedPrefix:ci_"`
	SelStorageProposalCid DBCid      `gorm:"column:sel_proposal_cid;type:varchar(256);"` // piece info
	Status                uint64     `gorm:"column:status;type:bigint unsigned;NOT NULL;"`
	Receiver              string     `gorm:"column:receiver;type:varchar(256);primary_key"`
	TotalSent             uint64     `gorm:"column:total_sent;type:bigint unsigned;NOT NULL;"`
	FundsReceived         mtypes.Int `gorm:"column:funds_received;type:varchar(256);default:0"`
	Message               string     `gorm:"column:message;type:varchar(2048);"`
	CurrentInterval       uint64     `gorm:"column:current_interval;type:bigint unsigned;NOT NULL;"`
	LegacyProtocol        bool       `gorm:"column:legacy_protocol;"`
	TimeStampOrm
}

type DealProposal struct {
	PayloadCID DBCid  `gorm:"column:payload_cid;type:varchar(256);"`
	ID         uint64 `gorm:"column:proposal_id;type:bigint unsigned;primary_key;"`

	Selector                *[]byte    `gorm:"column:selector;type:blob;"` // V1
	PieceCID                DBCid      `gorm:"column:piece_cid;type:varchar(256);"`
	PricePerByte            mtypes.Int `gorm:"column:price_perbyte;type:varchar(256);default:0"`
	PaymentInterval         uint64     `gorm:"column:payment_interval;type:bigint unsigned;NOT NULL;"` // when to request payment
	PaymentIntervalIncrease uint64     `gorm:"column:payment_interval_increase;type:bigint unsigned;NOT NULL;"`
	UnsealPrice             mtypes.Int `gorm:"column:unseal_price;type:varchar(256);default:0"`
}

func (m *retrievalDeal) TableName() string {
	return retrievalDealTableName
}

func fromProviderDealState(deal *types.ProviderDealState) (*retrievalDeal, error) {
	newdeal := &retrievalDeal{
		DealProposal: DealProposal{
			PayloadCID:              DBCid(deal.PayloadCID),
			ID:                      uint64(deal.ID),
			PricePerByte:            mtypes.SafeFromGo(deal.PricePerByte.Int),
			PaymentInterval:         deal.PaymentInterval,
			PaymentIntervalIncrease: deal.PaymentIntervalIncrease,
			UnsealPrice:             mtypes.SafeFromGo(deal.UnsealPrice.Int),
		},
		StoreID:               deal.StoreID,
		Status:                uint64(deal.Status),
		SelStorageProposalCid: DBCid(deal.SelStorageProposalCid),
		Receiver:              deal.Receiver.String(),
		TotalSent:             deal.TotalSent,
		FundsReceived:         mtypes.SafeFromGo(deal.FundsReceived.Int),
		Message:               deal.Message,
		CurrentInterval:       deal.CurrentInterval,
		LegacyProtocol:        deal.LegacyProtocol,
		TimeStampOrm:          TimeStampOrm{CreatedAt: deal.CreatedAt, UpdatedAt: deal.UpdatedAt},
	}
	if !deal.Selector.IsNull() {
		buf := &bytes.Buffer{}
		if err := deal.Selector.MarshalCBOR(buf); err != nil {
			return nil, err
		}
		bytes := buf.Bytes()
		newdeal.Selector = &bytes
	}
	if deal.ChannelID != nil {
		newdeal.ChannelID = ChannelID{
			Initiator: deal.ChannelID.Initiator.String(),
			Responder: deal.ChannelID.Responder.String(),
			ID:        uint64(deal.ChannelID.ID),
		}
	}

	if deal.DealProposal.PieceCID == nil {
		newdeal.DealProposal.PieceCID = UndefDBCid
	} else {
		newdeal.DealProposal.PieceCID = DBCid(*deal.DealProposal.PieceCID)
	}
	return newdeal, nil
}

func toProviderDealState(deal *retrievalDeal) (*types.ProviderDealState, error) {
	newdeal := &types.ProviderDealState{
		DealProposal: rm.DealProposal{
			PayloadCID: deal.PayloadCID.cid(),
			ID:         rm.DealID(deal.DealProposal.ID),
			Params: rm.Params{
				PieceCID:                deal.DealProposal.PieceCID.cidPtr(),
				PricePerByte:            abi.TokenAmount(mtypes.SafeFromGo(deal.PricePerByte.Int)),
				PaymentInterval:         deal.DealProposal.PaymentInterval,
				PaymentIntervalIncrease: deal.DealProposal.PaymentIntervalIncrease,
				UnsealPrice:             abi.TokenAmount(mtypes.SafeFromGo(deal.UnsealPrice.Int)),
			},
		},
		StoreID:               deal.StoreID,
		ChannelID:             nil,
		SelStorageProposalCid: deal.SelStorageProposalCid.cid(),
		Status:                rm.DealStatus(deal.Status),
		TotalSent:             deal.TotalSent,
		FundsReceived:         abi.TokenAmount(mtypes.SafeFromGo(deal.FundsReceived.Int)),
		Message:               deal.Message,
		CurrentInterval:       deal.CurrentInterval,
		LegacyProtocol:        deal.LegacyProtocol,
		TimeStamp:             deal.Timestamp(),
	}
	var err error

	if deal.Selector != nil {
		sel := &rm.CborGenCompatibleNode{}
		if err := sel.UnmarshalCBOR(bytes.NewBuffer(*deal.Selector)); err != nil {
			return nil, err
		}
		newdeal.DealProposal.Selector = *sel
	}

	if len(deal.Receiver) > 0 {
		newdeal.Receiver, err = decodePeerId(deal.Receiver)
		if err != nil {
			return nil, fmt.Errorf("decode receiver: %s", err)
		}
	}

	if len(deal.ChannelID.Initiator) > 0 {
		newdeal.ChannelID = &datatransfer.ChannelID{}
		newdeal.ChannelID.ID = datatransfer.TransferID(deal.ChannelID.ID)
		newdeal.ChannelID.Initiator, err = decodePeerId(deal.ChannelID.Initiator)
		if err != nil {
			return nil, fmt.Errorf("decode ci_initiator: %s", err)
		}
		newdeal.ChannelID.Responder, err = decodePeerId(deal.ChannelID.Responder)
		if err != nil {
			return nil, fmt.Errorf("decode ci_responder: %s", err)
		}
	}
	return newdeal, nil
}

type retrievalDealRepo struct {
	*gorm.DB
}

func (rdr *retrievalDealRepo) SaveDeal(ctx context.Context, deal *types.ProviderDealState) error {
	dbDeal, err := fromProviderDealState(deal)
	if err != nil {
		return err
	}
	dbDeal.TimeStampOrm.Refresh()
	return rdr.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).
		Create(dbDeal).Error
}

func (rdr *retrievalDealRepo) GetDeal(ctx context.Context, id peer.ID, id2 rm.DealID) (*types.ProviderDealState, error) {
	deal := &retrievalDeal{}
	err := rdr.WithContext(ctx).Table(retrievalDealTableName).Take(deal, "cdp_proposal_id=? AND receiver=? ", id2, id.String()).Error
	if err != nil {
		return nil, err
	}
	return toProviderDealState(deal)
}

func (rdr *retrievalDealRepo) GetDealByTransferId(ctx context.Context, chid datatransfer.ChannelID) (*types.ProviderDealState, error) {
	deal := &retrievalDeal{}
	err := rdr.WithContext(ctx).Table(retrievalDealTableName).Take(deal, "ci_initiator = ? AND ci_responder = ? AND ci_channel_id = ?", chid.Initiator.String(), chid.Responder.String(), chid.ID).Error
	if err != nil {
		return nil, err
	}
	return toProviderDealState(deal)
}

func (rdr *retrievalDealRepo) HasDeal(ctx context.Context, id peer.ID, id2 rm.DealID) (bool, error) {
	var count int64
	err := rdr.WithContext(ctx).Table(retrievalDealTableName).Where("cdp_proposal_id=? AND receiver=? ", id2, id.String()).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (rdr *retrievalDealRepo) ListDeals(ctx context.Context, params *types.RetrievalDealQueryParams) ([]*types.ProviderDealState, error) {
	var sqlMsgs []*retrievalDeal
	discardFailedDeal := params.DiscardFailedDeal
	if discardFailedDeal && params.Status != nil {
		discardFailedDeal = *params.Status != uint64(rm.DealStatusErrored)
	}

	query := rdr.DB.Table(retrievalDealTableName).Offset(params.Offset).Limit(params.Limit)
	if len(params.Receiver) > 0 {
		query.Where("receiver = ?", params.Receiver)
	}
	if len(params.PayloadCID) > 0 {
		query.Where("cdp_payload_cid = ?", params.PayloadCID)
	}
	if params.Status != nil {
		query.Where("status = ?", params.Status)
	}
	if discardFailedDeal {
		query.Where("status != ?", rm.DealStatusErrored)
	}

	if err := query.Find(&sqlMsgs).Error; err != nil {
		return nil, err
	}

	var err error
	result := make([]*types.ProviderDealState, len(sqlMsgs))
	for index, sqlMsg := range sqlMsgs {
		result[index], err = toProviderDealState(sqlMsg)
		if err != nil {
			return nil, err
		}
	}
	return result, err
}

// GroupRetrievalDealNumberByStatus Count the number of retrieval deal by status
// todo address undefined is invalid, it is currently not possible to directly associate an order with a miner
func (rdr *retrievalDealRepo) GroupRetrievalDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[rm.DealStatus]int64, error) {
	query := rdr.WithContext(ctx).Table(retrievalDealTableName).Group("state").Select("state, count(1) as count")
	var items []struct {
		State rm.DealStatus
		Count int64
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}

	result := map[rm.DealStatus]int64{}
	for _, item := range items {
		result[(item.State)] = item.Count
	}
	return result, nil
}

func NewRetrievalDealRepo(db *gorm.DB) repo.IRetrievalDealRepo {
	return &retrievalDealRepo{db}
}
