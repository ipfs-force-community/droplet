package mysql

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
	mtypes "github.com/filecoin-project/venus-messager/types"
	"github.com/ipfs/go-cid"
	typegen "github.com/whyrusleeping/cbor-gen"
	"gorm.io/gorm"
)

type storageDeal struct {
	ClientDealProposal `gorm:"embedded;embeddedPrefix:cdp_"`

	ProposalCid           string     `gorm:"column:proposal_cid;type:varchar(128);primary_key"`
	AddFundsCid           string     `gorm:"column:add_funds_cid;type:varchar(128);"`
	PublishCid            string     `gorm:"column:publish_cid;type:varchar(128);"`
	Miner                 string     `gorm:"column:miner_peer;type:varchar(128);"`
	Client                string     `gorm:"column:client_peer;type:varchar(128);"`
	State                 uint64     `gorm:"column:state;type:bigint unsigned;"`
	PiecePath             string     `gorm:"column:piece_path;type:varchar(128);"`
	MetadataPath          string     `gorm:"column:metadata_path;type:varchar(128);"`
	SlashEpoch            int64      `gorm:"column:slash_epoch;type:bigint;"`
	FastRetrieval         bool       `gorm:"column:fast_retrieval;"`
	Message               string     `gorm:"column:message;type:varchar(128);"`
	FundsReserved         mtypes.Int `gorm:"column:funds_reserved;type:varchar(256);"`
	Ref                   DataRef    `gorm:"embedded;embeddedPrefix:ref_"`
	AvailableForRetrieval bool       `gorm:"column:available_for_retrieval;"`

	DealID       uint64 `gorm:"column:deal_id;type:bigint unsigned;"`
	CreationTime int64  `gorm:"column:creation_time;type:bigint;"`

	TransferChannelId ChannelID `gorm:"embedded;embeddedPrefix:tci_"`
	SectorNumber      uint64    `gorm:"column:sector_number;type:bigint unsigned;"`

	InboundCAR string `gorm:"column:addr;type:varchar(128);primary_key"`
}

type ClientDealProposal struct {
	PieceCID     string `gorm:"column:addr;type:varchar(128);"`
	PieceSize    uint64 `gorm:"column:piece_size;type:bigint unsigned;"`
	VerifiedDeal bool   `gorm:"column:verified_deal;"`
	Client       string `gorm:"column:client;type:varchar(128);"`
	Provider     string `gorm:"column:provider;type:varchar(128);"`

	// Label is an arbitrary client chosen label to apply to the deal
	Label string `gorm:"column:label;type:varchar(256);"`

	// Nominal start epoch. Deal payment is linear between StartEpoch and EndEpoch,
	// with total amount StoragePricePerEpoch * (EndEpoch - StartEpoch).
	// Storage deal must appear in a sealed (proven) sector no later than StartEpoch,
	// otherwise it is invalid.
	StartEpoch           int64      `gorm:"column:start_epoch;type:bigint;"`
	EndEpoch             int64      `gorm:"column:end_epoch;type:bigint;"`
	StoragePricePerEpoch mtypes.Int `gorm:"column:storage_price_per_epoch;type:varchar(256);"`

	ProviderCollateral mtypes.Int `gorm:"column:provider_collateral;type:varchar(256);"`
	ClientCollateral   mtypes.Int `gorm:"column:client_collateral;type:varchar(256);"`

	ClientSignature Signature `gorm:"column:client_signature;type:blob;"`
}

type Signature acrypto.Signature

func (s *Signature) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value must be []byte")
	}
	return json.Unmarshal(b, s)
}

func (s Signature) Value() (driver.Value, error) {
	return json.Marshal(s)
}

type ChannelID struct {
	Initiator string `gorm:"column:initiator;type:varchar(256);"`
	Responder string `gorm:"column:responder;type:varchar(256);"`
	ID        uint64 `gorm:"column:channel_id;type:bigint unsigned;"`
}

type DataRef struct {
	TransferType string `gorm:"column:transfer_type;type:varchar(128);"`
	Root         string `gorm:"column:root;type:varchar(128);"`

	PieceCid     string                `gorm:"column:piece_cid;type:varchar(256);"`
	PieceSize    abi.UnpaddedPieceSize `gorm:"column:piece_size;type:bigint unsigned;"`
	RawBlockSize uint64                `gorm:"column:raw_block_size;type:bigint unsigned;"`
}

func (m *storageDeal) TableName() string {
	return "storage_deals"
}

func fromStorageDeal(src *storagemarket.MinerDeal) *storageDeal {
	md := &storageDeal{
		ClientDealProposal: ClientDealProposal{
			PieceCID:             decodeCid(src.ClientDealProposal.Proposal.PieceCID),
			PieceSize:            uint64(src.ClientDealProposal.Proposal.PieceSize),
			VerifiedDeal:         src.ClientDealProposal.Proposal.VerifiedDeal,
			Client:               src.ClientDealProposal.Proposal.Client.String(),
			Provider:             src.ClientDealProposal.Proposal.Provider.String(),
			Label:                src.ClientDealProposal.Proposal.Label,
			StartEpoch:           int64(src.ClientDealProposal.Proposal.StartEpoch),
			EndEpoch:             int64(src.ClientDealProposal.Proposal.EndEpoch),
			StoragePricePerEpoch: convertBigInt(src.ClientDealProposal.Proposal.StoragePricePerEpoch),
			ProviderCollateral:   convertBigInt(src.ClientDealProposal.Proposal.ProviderCollateral),
			ClientCollateral:     convertBigInt(src.ClientDealProposal.Proposal.ClientCollateral),
			ClientSignature: Signature{
				Type: src.ClientSignature.Type,
				Data: src.ClientSignature.Data,
			},
		},
		ProposalCid:           decodeCid(src.ProposalCid),
		AddFundsCid:           decodeCidPtr(src.AddFundsCid),
		PublishCid:            decodeCidPtr(src.PublishCid),
		Miner:                 src.Miner.Pretty(),
		Client:                src.Client.Pretty(),
		State:                 src.State,
		PiecePath:             string(src.PiecePath),
		MetadataPath:          string(src.MetadataPath),
		SlashEpoch:            int64(src.SlashEpoch),
		FastRetrieval:         src.FastRetrieval,
		Message:               src.Message,
		FundsReserved:         convertBigInt(src.FundsReserved),
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                uint64(src.DealID),
		CreationTime:          src.CreationTime.Time().UnixNano(),
		SectorNumber:          uint64(src.SectorNumber),
		InboundCAR:            src.InboundCAR,
	}

	if src.Ref != nil {
		md.Ref = DataRef{
			TransferType: src.Ref.TransferType,
			Root:         decodeCid(src.Ref.Root),
			PieceCid:     decodeCidPtr(src.Ref.PieceCid),
			PieceSize:    src.Ref.PieceSize,
			RawBlockSize: src.Ref.RawBlockSize,
		}
	}
	if src.TransferChannelId != nil {
		md.TransferChannelId = ChannelID{
			Initiator: src.TransferChannelId.Initiator.String(),
			Responder: src.TransferChannelId.Responder.String(),
			ID:        uint64(src.TransferChannelId.ID),
		}
	}

	return md
}

func toStorageDeal(src *storageDeal) (*storagemarket.MinerDeal, error) {
	md := &storagemarket.MinerDeal{
		ClientDealProposal: market.ClientDealProposal{
			Proposal: market.DealProposal{
				PieceSize:            abi.PaddedPieceSize(src.PieceSize),
				VerifiedDeal:         src.VerifiedDeal,
				Label:                src.Label,
				StartEpoch:           abi.ChainEpoch(src.StartEpoch),
				EndEpoch:             abi.ChainEpoch(src.EndEpoch),
				StoragePricePerEpoch: abi.TokenAmount{Int: src.StoragePricePerEpoch.Int},
				ProviderCollateral:   abi.TokenAmount{Int: src.ProviderCollateral.Int},
				ClientCollateral:     abi.TokenAmount{Int: src.ClientCollateral.Int},
			},
			ClientSignature: acrypto.Signature{
				Type: src.ClientSignature.Type,
				Data: src.ClientSignature.Data,
			},
		},
		State:         src.State,
		PiecePath:     filestore.Path(src.PiecePath),
		MetadataPath:  filestore.Path(src.MetadataPath),
		SlashEpoch:    abi.ChainEpoch(src.SlashEpoch),
		FastRetrieval: src.FastRetrieval,
		Message:       src.Message,
		FundsReserved: abi.TokenAmount{Int: src.FundsReserved.Int},
		Ref: &storagemarket.DataRef{
			TransferType: src.Ref.TransferType,
			PieceSize:    src.Ref.PieceSize,
			RawBlockSize: src.Ref.RawBlockSize,
		},
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                abi.DealID(src.DealID),
		CreationTime:          typegen.CborTime(time.Unix(0, src.CreationTime).UTC()),
		SectorNumber:          abi.SectorNumber(src.SectorNumber),
		InboundCAR:            src.InboundCAR,
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
	md.AddFundsCid, err = parseCidPtr(src.AddFundsCid)
	if err != nil {
		return nil, err
	}
	md.PublishCid, err = parseCidPtr(src.PublishCid)
	if err != nil {
		return nil, err
	}
	md.Ref.Root, err = parseCid(src.Ref.Root)
	if err != nil {
		return nil, err
	}
	md.Ref.PieceCid, err = parseCidPtr(src.Ref.PieceCid)
	if err != nil {
		return nil, err
	}

	if len(src.TransferChannelId.Initiator) > 0 {
		md.TransferChannelId = &datatransfer.ChannelID{}
		md.TransferChannelId.ID = datatransfer.TransferID(src.TransferChannelId.ID)
		md.TransferChannelId.Initiator, err = decodePeerId(src.TransferChannelId.Initiator)
		if err != nil {
			return nil, err
		}
		md.TransferChannelId.Responder, err = decodePeerId(src.TransferChannelId.Responder)
		if err != nil {
			return nil, err
		}
	}

	md.Miner, err = decodePeerId(src.Miner)
	if err != nil {
		return nil, err
	}
	md.Client, err = decodePeerId(src.Client)
	if err != nil {
		return nil, err
	}

	return md, nil
}

type storageDealRepo struct {
	*gorm.DB
}

func NewStorageDealRepo(db *gorm.DB) *storageDealRepo {
	return &storageDealRepo{db}
}

func (m *storageDealRepo) SaveStorageDeal(StorageDeal *storagemarket.MinerDeal) error {
	return m.DB.Save(fromStorageDeal(StorageDeal)).Error
}

func (m *storageDealRepo) GetStorageDeal(proposalCid cid.Cid) (*storagemarket.MinerDeal, error) {
	var md storageDeal
	err := m.DB.Take(&md, "proposal_cid = ?", proposalCid.String()).Error
	if err != nil {
		return nil, err
	}

	return toStorageDeal(&md)
}

func (m *storageDealRepo) ListStorageDeal() ([]*storagemarket.MinerDeal, error) {
	var mds []*storageDeal
	err := m.DB.Find(&mds).Error
	if err != nil {
		return nil, err
	}
	list := make([]*storagemarket.MinerDeal, 0, len(mds))
	for _, md := range mds {
		deal, err := toStorageDeal(md)
		if err != nil {
			return nil, err
		}
		list = append(list, deal)
	}

	return list, nil
}
