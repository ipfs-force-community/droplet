package storagemysql

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/venus-market/types"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	typegen "github.com/whyrusleeping/cbor-gen"
	"gorm.io/gorm"
)

type minerDeal struct {
	ClientDealProposal `gorm:"embedded;embeddedPrefix:cdp_"`

	ProposalCid           string                          `gorm:"column:proposal_cid;type:varchar(256);primary_key"`
	AddFundsCid           string                          `gorm:"column:add_funds_cid;type:varchar(256);"`
	PublishCid            string                          `gorm:"column:publish_cid;type:varchar(256);"`
	Miner                 peer.ID                         `gorm:"column:miner_peer;type:varchar(128);"`
	Client                peer.ID                         `gorm:"column:client_peer;type:varchar(128);"`
	State                 storagemarket.StorageDealStatus `gorm:"column:state;type:bigint unsigned;"`
	PiecePath             filestore.Path                  `gorm:"column:piece_path;type:varchar(128);"`
	MetadataPath          filestore.Path                  `gorm:"column:metadata_path;type:varchar(128);"`
	SlashEpoch            abi.ChainEpoch                  `gorm:"column:slash_epoch;type:bigint;"`
	FastRetrieval         bool                            `gorm:"column:fast_retrieval;"`
	Message               string                          `gorm:"column:message;type:varchar(128);"`
	FundsReserved         mtypes.Int                      `gorm:"column:funds_reserved;type:varchar(256);"`
	Ref                   DataRef                         `gorm:"embedded;embeddedPrefix:ref_"`
	AvailableForRetrieval bool                            `gorm:"column:available_for_retrieval;"`

	DealID       abi.DealID `gorm:"column:deal_id;type:bigint unsigned;"`
	CreationTime uint64     `gorm:"column:creation_time;type:bigint unsigned;"`

	TransferChannelId datatransfer.ChannelID `gorm:"embedded;embeddedPrefix:tci_"`
	SectorNumber      abi.SectorNumber       `gorm:"column:sector_number;type:bigint unsigned;"`

	InboundCAR string `gorm:"column:addr;type:varchar(256);primary_key"`
}

type ClientDealProposal struct {
	PieceCID     string              `gorm:"column:addr;type:varchar(256);"`
	PieceSize    abi.PaddedPieceSize `gorm:"column:piece_size;type:bigint unsigned;"`
	VerifiedDeal bool                `gorm:"column:verified_deal;"`
	Client       string              `gorm:"column:client;type:varchar(256);"`
	Provider     string              `gorm:"column:provider;type:varchar(256);"`

	// Label is an arbitrary client chosen label to apply to the deal
	Label string `gorm:"column:addr;type:varchar(256);"`

	// Nominal start epoch. Deal payment is linear between StartEpoch and EndEpoch,
	// with total amount StoragePricePerEpoch * (EndEpoch - StartEpoch).
	// Storage deal must appear in a sealed (proven) sector no later than StartEpoch,
	// otherwise it is invalid.
	StartEpoch           abi.ChainEpoch `gorm:"column:start_epoch;type:bigint;"`
	EndEpoch             abi.ChainEpoch `gorm:"column:end_epoch;type:bigint;"`
	StoragePricePerEpoch mtypes.Int     `gorm:"column:storage_price_per_epoch;type:varchar(256);"`

	ProviderCollateral mtypes.Int `gorm:"column:provider_collateral;type:varchar(256);"`
	ClientCollateral   mtypes.Int `gorm:"column:client_collateral;type:varchar(256);"`

	ClientSignature Signature `gorm:"column:client_signature;type:blob;"`
}

type Signature acrypto.Signature

func (s *Signature) Scan(value interface{}) error {
	sqlBin, isok := value.([]byte)
	if !isok {
		return fmt.Errorf("value must be []byte")
	}
	return json.Unmarshal(sqlBin, s)
}

func (s *Signature) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

type DataRef struct {
	TransferType string `gorm:"column:transfer_type;type:varchar(128);"`
	Root         string `gorm:"column:root;type:varchar(128);"`

	PieceCid     string                `gorm:"column:piece_cid;type:varchar(256);"`
	PieceSize    abi.UnpaddedPieceSize `gorm:"column:piece_size;type:bigint unsigned;"`
	RawBlockSize uint64                `gorm:"column:raw_block_size;type:bigint unsigned;"`
}

func (m *minerDeal) TableName() string {
	return "miner_deals"
}

func fromMinerDeal(src *types.MinerDeal) *minerDeal {
	md := &minerDeal{
		ClientDealProposal:    ClientDealProposal{},
		ProposalCid:           src.ProposalCid.String(),
		AddFundsCid:           src.AddFundsCid.String(),
		PublishCid:            src.PublishCid.String(),
		Miner:                 src.Miner,
		Client:                src.Client,
		State:                 src.State,
		PiecePath:             src.PiecePath,
		MetadataPath:          src.MetadataPath,
		SlashEpoch:            src.SlashEpoch,
		FastRetrieval:         src.FastRetrieval,
		Message:               src.Message,
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                src.DealID,
		CreationTime:          uint64(src.CreationTime.Time().Unix()),
		SectorNumber:          src.SectorNumber,
		InboundCAR:            src.InboundCAR,
	}
	if !src.FundsReserved.Nil() {
		md.FundsReserved = mtypes.NewFromGo(src.FundsReserved.Int)
	} else {
		md.FundsReserved = mtypes.Zero()
	}
	if src.Ref != nil {
		md.Ref = DataRef{
			TransferType: src.Ref.TransferType,
			Root:         src.Ref.Root.String(),
			PieceSize:    src.Ref.PieceSize,
			RawBlockSize: src.Ref.RawBlockSize,
		}
		if src.Ref.PieceCid != nil {
			md.Ref.PieceCid = src.Ref.PieceCid.String()
		}
	}
	if src.TransferChannelId != nil {
		md.TransferChannelId = datatransfer.ChannelID{
			Initiator: src.TransferChannelId.Initiator,
			Responder: src.TransferChannelId.Responder,
			ID:        src.TransferChannelId.ID,
		}
	}

	return md
}

func toMinerDeal(src *minerDeal) (*types.MinerDeal, error) {
	md := &types.MinerDeal{
		ClientDealProposal: market.ClientDealProposal{
			Proposal: market.DealProposal{
				PieceSize:            src.PieceSize,
				VerifiedDeal:         src.VerifiedDeal,
				Label:                src.Label,
				StartEpoch:           src.StartEpoch,
				EndEpoch:             src.EndEpoch,
				StoragePricePerEpoch: abi.TokenAmount{Int: src.StoragePricePerEpoch.Int},
				ProviderCollateral:   abi.TokenAmount{Int: src.ProviderCollateral.Int},
				ClientCollateral:     abi.TokenAmount{Int: src.ClientCollateral.Int},
			},
			ClientSignature: acrypto.Signature{
				Type: src.ClientSignature.Type,
				Data: src.ClientSignature.Data,
			},
		},
		Miner:         src.Miner,
		Client:        src.Client,
		State:         src.State,
		PiecePath:     src.PiecePath,
		MetadataPath:  src.MetadataPath,
		SlashEpoch:    src.SlashEpoch,
		FastRetrieval: src.FastRetrieval,
		Message:       src.Message,
		FundsReserved: abi.TokenAmount{Int: src.FundsReserved.Int},
		Ref: &storagemarket.DataRef{
			TransferType: src.Ref.TransferType,
			PieceSize:    src.Ref.PieceSize,
			RawBlockSize: src.Ref.RawBlockSize,
		},
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                src.DealID,
		CreationTime:          typegen.CborTime(time.Unix(int64(src.CreationTime), 0)),
		TransferChannelId: &datatransfer.ChannelID{
			Initiator: src.TransferChannelId.Initiator,
			Responder: src.TransferChannelId.Responder,
			ID:        src.TransferChannelId.ID,
		},
		SectorNumber: src.SectorNumber,
		InboundCAR:   src.InboundCAR,
	}
	var err error
	md.ClientDealProposal.Proposal.PieceCID, err = parseCid(src.ClientDealProposal.PieceCID)
	if err != nil {
		return nil, err
	}
	md.ClientDealProposal.Proposal.Client, err = address.NewFromString(src.ClientDealProposal.Client)
	if err != nil {
		return nil, err
	}
	md.ClientDealProposal.Proposal.Provider, err = address.NewFromString(src.ClientDealProposal.Provider)
	if err != nil {
		return nil, err
	}
	md.ProposalCid, err = parseCid(src.ProposalCid)
	if err != nil {
		return nil, err
	}
	addFundsCid, err := parseCid(src.AddFundsCid)
	if err != nil {
		return nil, err
	}
	md.AddFundsCid = &addFundsCid
	publishCid, err := parseCid(src.PublishCid)
	if err != nil {
		return nil, err
	}
	md.PublishCid = &publishCid
	root, err := parseCid(src.Ref.Root)
	if err != nil {
		return nil, err
	}
	md.Ref.Root = root
	pieceCid, err := parseCid(src.Ref.PieceCid)
	if err != nil {
		return nil, err
	}
	md.Ref.PieceCid = &pieceCid

	return md, nil
}

func parseCid(str string) (cid.Cid, error) {
	if len(str) == 0 {
		return cid.Undef, nil
	}

	return cid.Parse(str)
}

type minerDealRepo struct {
	*gorm.DB
}

func newMinerDealRepo(db *gorm.DB) *minerDealRepo {
	return &minerDealRepo{db}
}

func (m *minerDealRepo) CreateMinerDeal(minerDeal *types.MinerDeal) error {
	return m.DB.Create(fromMinerDeal(minerDeal)).Error
}

func (m *minerDealRepo) GetDeal(proposalCid cid.Cid) (*types.MinerDeal, error) {
	var md minerDeal
	err := m.DB.Take(md, "proposal_cid = ?", proposalCid.String()).Error
	if err != nil {
		return nil, err
	}

	return toMinerDeal(&md)
}

func (m *minerDealRepo) UpdateDeal(proposalCid cid.Cid, updateCols map[string]interface{}) error {
	return m.DB.Model(&minerDeal{}).Where("proposal_cid = ?", proposalCid.String()).UpdateColumns(updateCols).Error
}

func (m *minerDealRepo) ListMinerDeal() ([]*types.MinerDeal, error) {
	var mds []*minerDeal
	err := m.DB.Find(&mds).Error
	if err != nil {
		return nil, err
	}
	list := make([]*types.MinerDeal, 0, len(mds))
	for _, md := range mds {
		deal, err := toMinerDeal(md)
		if err != nil {
			return nil, err
		}
		list = append(list, deal)
	}

	return list, nil
}

var _ MinerDealRepo = (*minerDealRepo)(nil)
