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
	"github.com/libp2p/go-libp2p-core/peer"
	typegen "github.com/whyrusleeping/cbor-gen"
	"gorm.io/gorm"
)

type storageDeal struct {
	ClientDealProposal `gorm:"embedded;embeddedPrefix:cdp_"`

	ProposalCid           string                          `gorm:"column:proposal_cid;type:varchar(128);primary_key"`
	AddFundsCid           string                          `gorm:"column:add_funds_cid;type:varchar(128);"`
	PublishCid            string                          `gorm:"column:publish_cid;type:varchar(128);"`
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
	CreationTime int64      `gorm:"column:creation_time;type:bigint;"`

	TransferChannelId datatransfer.ChannelID `gorm:"embedded;embeddedPrefix:tci_"`
	SectorNumber      abi.SectorNumber       `gorm:"column:sector_number;type:bigint unsigned;"`

	InboundCAR string `gorm:"column:addr;type:varchar(128);primary_key"`
}

type ClientDealProposal struct {
	PieceCID     string              `gorm:"column:addr;type:varchar(128);"`
	PieceSize    abi.PaddedPieceSize `gorm:"column:piece_size;type:bigint unsigned;"`
	VerifiedDeal bool                `gorm:"column:verified_deal;"`
	Client       string              `gorm:"column:client;type:varchar(128);"`
	Provider     string              `gorm:"column:provider;type:varchar(128);"`

	// Label is an arbitrary client chosen label to apply to the deal
	Label string `gorm:"column:label;type:varchar(256);"`

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
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value must be []byte")
	}
	return json.Unmarshal(b, s)
}

func (s Signature) Value() (driver.Value, error) {
	return json.Marshal(s)
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
			PieceSize:            src.ClientDealProposal.Proposal.PieceSize,
			VerifiedDeal:         src.ClientDealProposal.Proposal.VerifiedDeal,
			Client:               src.ClientDealProposal.Proposal.Client.String(),
			Provider:             src.ClientDealProposal.Proposal.Provider.String(),
			Label:                src.ClientDealProposal.Proposal.Label,
			StartEpoch:           src.ClientDealProposal.Proposal.StartEpoch,
			EndEpoch:             src.ClientDealProposal.Proposal.EndEpoch,
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
		Miner:                 src.Miner,
		Client:                src.Client,
		State:                 src.State,
		PiecePath:             src.PiecePath,
		MetadataPath:          src.MetadataPath,
		SlashEpoch:            src.SlashEpoch,
		FastRetrieval:         src.FastRetrieval,
		Message:               src.Message,
		FundsReserved:         convertBigInt(src.FundsReserved),
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                src.DealID,
		CreationTime:          src.CreationTime.Time().UnixNano(),
		SectorNumber:          src.SectorNumber,
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
		md.TransferChannelId = *src.TransferChannelId
	}

	return md
}

func toStorageDeal(src *storageDeal) (*storagemarket.MinerDeal, error) {
	md := &storagemarket.MinerDeal{
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
		CreationTime:          typegen.CborTime(time.Unix(0, src.CreationTime).UTC()),
		SectorNumber:          src.SectorNumber,
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

	channelID := src.TransferChannelId
	if len(channelID.Initiator) == 0 && len(channelID.Responder) == 0 && channelID.ID == 0 {
		md.TransferChannelId = nil
	} else {
		md.TransferChannelId = &datatransfer.ChannelID{
			Initiator: src.TransferChannelId.Initiator,
			Responder: src.TransferChannelId.Responder,
			ID:        src.TransferChannelId.ID,
		}
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
