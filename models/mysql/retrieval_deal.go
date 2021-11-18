package mysql

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/models/repo"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
	"gorm.io/gorm"
)

const RetrievalDealTableName = "retrieval_deals"

type retrievalDeal struct {
	DealProposal    `gorm:"embedded;embeddedPrefix:cdp_"`
	StoreID         uint64     `gorm:"column:store_id;type:bigint unsigned;"`
	ChannelID       ChannelID  `gorm:"embedded;embeddedPrefix:ci_"`
	PieceCID        string     `gorm:"column:piece_cid;type:varchar(256);"` //piece info
	Status          uint64     `gorm:"column:status;type:bigint unsigned;"`
	Receiver        string     `gorm:"column:receiver;type:varchar(256);primary_key"`
	TotalSent       uint64     `gorm:"column:total_sent;type:bigint unsigned;"`
	FundsReceived   mtypes.Int `gorm:"column:funds_received;type:varchar(256);"`
	Message         string     `gorm:"column:message;type:varchar(2048);"`
	CurrentInterval uint64     `gorm:"column:current_interval;type:bigint unsigned;"`
	LegacyProtocol  bool       `gorm:"column:legacy_protocol;"`
}

type DealProposal struct {
	PayloadCID string `gorm:"column:payload_cid;type:varchar(256);"`
	ID         uint64 `gorm:"column:proposal_id;type:bigint unsigned;primary_key"`

	Selector                *[]byte    `gorm:"column:selector;type:blob;"` // V1
	PricePerByte            mtypes.Int `gorm:"column:price_perbyte;type:varchar(256);"`
	PaymentInterval         uint64     `gorm:"column:payment_interval;type:bigint unsigned;"` // when to request payment
	PaymentIntervalIncrease uint64     `gorm:"column:payment_interval_increase;type:bigint unsigned;"`
	UnsealPrice             mtypes.Int `gorm:"column:unseal_price;type:varchar(256);"`
}

func (m *retrievalDeal) TableName() string {
	return RetrievalDealTableName
}

func fromProviderDealState(deal *rm.ProviderDealState) *retrievalDeal {
	newdeal := &retrievalDeal{
		DealProposal: DealProposal{
			PayloadCID:              deal.PayloadCID.String(),
			ID:                      uint64(deal.ID),
			PricePerByte:            mtypes.Int(deal.PricePerByte),
			PaymentInterval:         deal.PaymentInterval,
			PaymentIntervalIncrease: deal.PaymentIntervalIncrease,
			UnsealPrice:             mtypes.Int(deal.UnsealPrice),
		},
		StoreID:         deal.StoreID,
		PieceCID:        deal.PieceCID.String(),
		Status:          uint64(deal.Status),
		Receiver:        deal.Receiver.String(),
		TotalSent:       deal.TotalSent,
		FundsReceived:   mtypes.Int(deal.FundsReceived),
		Message:         deal.Message,
		CurrentInterval: deal.CurrentInterval,
		LegacyProtocol:  deal.LegacyProtocol,
	}
	deal.Identifier()
	if deal.Selector != nil {
		newdeal.Selector = &deal.Selector.Raw
	}
	if deal.ChannelID != nil {
		newdeal.ChannelID = ChannelID{
			Initiator: deal.ChannelID.Initiator.String(),
			Responder: deal.ChannelID.Responder.String(),
			ID:        uint64(deal.ChannelID.ID),
		}
	}
	if deal.PieceCID != nil {
		newdeal.PieceCID = deal.PieceCID.String()
	}
	return newdeal
}

func toProviderDealState(deal *retrievalDeal) (*rm.ProviderDealState, error) {
	newdeal := &rm.ProviderDealState{
		DealProposal: rm.DealProposal{
			ID: rm.DealID(deal.DealProposal.ID),
			Params: rm.Params{
				PricePerByte:            abi.TokenAmount(deal.PricePerByte),
				PaymentInterval:         deal.DealProposal.PaymentInterval,
				PaymentIntervalIncrease: deal.DealProposal.PaymentIntervalIncrease,
				UnsealPrice:             abi.TokenAmount(deal.UnsealPrice),
			},
		},
		StoreID:         deal.StoreID,
		ChannelID:       nil,
		Status:          rm.DealStatus(deal.Status),
		TotalSent:       deal.TotalSent,
		FundsReceived:   abi.TokenAmount(deal.FundsReceived),
		Message:         deal.Message,
		CurrentInterval: deal.CurrentInterval,
		LegacyProtocol:  deal.LegacyProtocol,
	}

	var err error
	if len(deal.DealProposal.PayloadCID) > 0 {
		newdeal.DealProposal.PayloadCID, err = parseCid(deal.DealProposal.PayloadCID)
		if err != nil {
			return nil, err
		}
	}
	if deal.DealProposal.Selector != nil {
		newdeal.DealProposal.Selector = &cbg.Deferred{Raw: *deal.Selector}
	}

	if len(deal.PieceCID) > 0 {
		pieceCid, err := parseCid(deal.PieceCID)
		if err != nil {
			return nil, err
		}
		newdeal.DealProposal.PayloadCID = pieceCid
		newdeal.PieceInfo = &piecestore.PieceInfo{PieceCID: pieceCid}
	}

	if len(deal.Receiver) > 0 {
		newdeal.Receiver, err = decodePeerId(deal.Receiver)
		if err != nil {
			return nil, err
		}
	}

	if len(deal.ChannelID.Initiator) > 0 {
		newdeal.ChannelID = &datatransfer.ChannelID{}
		newdeal.ChannelID.ID = datatransfer.TransferID(deal.ChannelID.ID)
		newdeal.ChannelID.Initiator, err = decodePeerId(deal.ChannelID.Initiator)
		if err != nil {
			return nil, err
		}
		newdeal.ChannelID.Responder, err = decodePeerId(deal.ChannelID.Responder)
		if err != nil {
			return nil, err
		}
	}
	return newdeal, nil
}

type retrievalDealRepo struct {
	*gorm.DB
}

func (r *retrievalDealRepo) SaveDeal(deal *rm.ProviderDealState) error {
	dbDeal := fromProviderDealState(deal)
	return r.Save(dbDeal).Error
}

func (r *retrievalDealRepo) GetDeal(id peer.ID, id2 rm.DealID) (*rm.ProviderDealState, error) {
	deal := &retrievalDeal{}
	err := r.Take(RetrievalDealTableName).Take(deal, "cdp_proposal_id=? and receiver=? ", id2, id.String()).Error
	if err != nil {
		return nil, err
	}
	return toProviderDealState(deal)
}

func (r *retrievalDealRepo) HasDeal(id peer.ID, id2 rm.DealID) (bool, error) {
	var count int64
	err := r.DB.Table(RetrievalDealTableName).Where("cdp_proposal_id=? and receiver=? ", id2, id.String()).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *retrievalDealRepo) ListDeals(pageIndex, pageSize int) ([]*rm.ProviderDealState, error) {
	query := r.DB.Table(RetrievalDealTableName).Offset((pageIndex - 1) * pageSize).Limit(pageSize)

	var sqlMsgs []*retrievalDeal
	err := query.Find(&sqlMsgs).Error
	if err != nil {
		return nil, err
	}

	result := make([]*rm.ProviderDealState, len(sqlMsgs))
	for index, sqlMsg := range sqlMsgs {
		result[index], err = toProviderDealState(sqlMsg)
		if err != nil {
			return nil, err
		}
	}
	return result, err
}

func NewRetrievalDealRepo(db *gorm.DB) repo.IRetrievalDealRepo {
	return &retrievalDealRepo{db}
}
