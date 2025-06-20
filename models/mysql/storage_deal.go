package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/sophon-messager/models/mtypes"
	"github.com/ipfs/go-cid"
	typegen "github.com/whyrusleeping/cbor-gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const storageDealTableName = "storage_deals"

type storageDeal struct {
	ID                 string `gorm:"column:id;type:varchar(128);index"`
	ClientDealProposal `gorm:"embedded;embeddedPrefix:cdp_"`

	ProposalCid DBCid  `gorm:"column:proposal_cid;type:varchar(256);primary_key"`
	AddFundsCid DBCid  `gorm:"column:add_funds_cid;type:varchar(256);"`
	PublishCid  DBCid  `gorm:"column:publish_cid;type:varchar(256);"`
	Miner       string `gorm:"column:miner_peer;type:varchar(128);index:miner_state"`
	Client      string `gorm:"column:client_peer;type:varchar(128);"`
	State       uint64 `gorm:"column:state;type:bigint unsigned;index;index:miner_state;index:idx_cdpprovider_state;NOT NULL;"`

	PayloadSize           uint64     `gorm:"column:payload_size;type:bigint;NOT NULL;"`
	PiecePath             string     `gorm:"column:piece_path;type:varchar(256);"`
	MetadataPath          string     `gorm:"column:metadata_path;type:varchar(256);"`
	SlashEpoch            int64      `gorm:"column:slash_epoch;type:bigint;NOT NULL;"`
	FastRetrieval         bool       `gorm:"column:fast_retrieval;"`
	Message               string     `gorm:"column:message;type:varchar(512);"`
	FundsReserved         mtypes.Int `gorm:"column:funds_reserved;type:varchar(256);default:0"`
	Ref                   DataRef    `gorm:"embedded;embeddedPrefix:ref_"`
	AvailableForRetrieval bool       `gorm:"column:available_for_retrieval;"`

	DealID       uint64 `gorm:"column:deal_id;type:bigint unsigned;index;NOT NULL;"`
	CreationTime int64  `gorm:"column:creation_time;type:bigint;NOT NULL;"`

	TransferChannelId ChannelID `gorm:"embedded;embeddedPrefix:tci_"`
	SectorNumber      uint64    `gorm:"column:sector_number;type:bigint unsigned;NOT NULL;"`

	InboundCAR string `gorm:"column:addr;type:varchar(256);"`

	Offset      uint64 `gorm:"column:offset;type:bigint;NOT NULL;"`
	Length      uint64 `gorm:"column:length;type:bigint;NOT NULL;"`
	PieceStatus string `gorm:"column:piece_status;type:varchar(128);index"`

	TimeStampOrm
}

type ClientDealProposal struct {
	PieceCID     DBCid     `gorm:"column:piece_cid;type:varchar(256);index"`
	PieceSize    uint64    `gorm:"column:piece_size;type:bigint unsigned;NOT NULL;"`
	VerifiedDeal bool      `gorm:"column:verified_deal;"`
	Client       DBAddress `gorm:"column:client;type:varchar(256);"`
	Provider     DBAddress `gorm:"column:provider;type:varchar(256);index;index:idx_cdpprovider_state"`

	// Label is an arbitrary client chosen label to apply to the deal
	Label string `gorm:"column:label;type:varchar(256);"`

	// Nominal start epoch. Deal payment is linear between StartEpoch and EndEpoch,
	// with total amount StoragePricePerEpoch * (EndEpoch - StartEpoch).
	// Storage deal must appear in a sealed (proven) sector no later than StartEpoch,
	// otherwise it is invalid.
	StartEpoch           int64      `gorm:"column:start_epoch;type:bigint;NOT NULL;"`
	EndEpoch             int64      `gorm:"column:end_epoch;type:bigint;NOT NULL;"`
	StoragePricePerEpoch mtypes.Int `gorm:"column:storage_price_per_epoch;type:varchar(256);default:0"`

	ProviderCollateral mtypes.Int `gorm:"column:provider_collateral;type:varchar(256);default:0"`
	ClientCollateral   mtypes.Int `gorm:"column:client_collateral;type:varchar(256);default:0"`

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
	ID        uint64 `gorm:"column:channel_id;type:bigint unsigned;NOT NULL;"`
}

type DataRef struct {
	TransferType string `gorm:"column:transfer_type;type:varchar(128);"`
	Root         DBCid  `gorm:"column:root;type:varchar(256);"`

	// todo remove filed below
	PieceCid     DBCid                 `gorm:"column:piece_cid;type:varchar(256);"`
	PieceSize    abi.UnpaddedPieceSize `gorm:"column:piece_size;type:bigint unsigned;NOT NULL;"`
	RawBlockSize uint64                `gorm:"column:raw_block_size;type:bigint unsigned;NOT NULL;"`
}

func (m *storageDeal) TableName() string {
	return storageDealTableName
}

func fromStorageDeal(src *types.MinerDeal) *storageDeal {
	label := src.ClientDealProposal.Proposal.Label
	labelStr := ""
	if label.IsString() {
		labelStr, _ = label.ToString()
	} else {
		labelBytes, _ := label.ToBytes()
		labelStr = string(labelBytes)
	}
	md := &storageDeal{
		ID: src.ID.String(),
		ClientDealProposal: ClientDealProposal{
			PieceCID:             DBCid(src.ClientDealProposal.Proposal.PieceCID),
			PieceSize:            uint64(src.ClientDealProposal.Proposal.PieceSize),
			VerifiedDeal:         src.ClientDealProposal.Proposal.VerifiedDeal,
			Client:               DBAddress(src.ClientDealProposal.Proposal.Client),
			Provider:             DBAddress(src.ClientDealProposal.Proposal.Provider),
			Label:                labelStr,
			StartEpoch:           int64(src.ClientDealProposal.Proposal.StartEpoch),
			EndEpoch:             int64(src.ClientDealProposal.Proposal.EndEpoch),
			StoragePricePerEpoch: mtypes.SafeFromGo(src.ClientDealProposal.Proposal.StoragePricePerEpoch.Int),
			ProviderCollateral:   mtypes.SafeFromGo(src.ClientDealProposal.Proposal.ProviderCollateral.Int),
			ClientCollateral:     mtypes.SafeFromGo(src.ClientDealProposal.Proposal.ClientCollateral.Int),
			ClientSignature: Signature{
				Type: src.ClientSignature.Type,
				Data: src.ClientSignature.Data,
			},
		},
		ProposalCid:           DBCid(src.ProposalCid),
		Miner:                 src.Miner.String(),
		Client:                src.Client.String(),
		State:                 src.State,
		PayloadSize:           src.PayloadSize,
		PiecePath:             string(src.PiecePath),
		MetadataPath:          string(src.MetadataPath),
		SlashEpoch:            int64(src.SlashEpoch),
		FastRetrieval:         src.FastRetrieval,
		Message:               src.Message,
		FundsReserved:         mtypes.SafeFromGo(src.FundsReserved.Int),
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                uint64(src.DealID),
		CreationTime:          src.CreationTime.Time().UnixNano(),
		SectorNumber:          uint64(src.SectorNumber),
		InboundCAR:            src.InboundCAR,

		Offset:       uint64(src.Offset),
		Length:       uint64(src.Proposal.PieceSize),
		PieceStatus:  string(src.PieceStatus),
		TimeStampOrm: TimeStampOrm{CreatedAt: src.CreatedAt, UpdatedAt: src.UpdatedAt},
	}

	if src.AddFundsCid == nil {
		md.AddFundsCid = UndefDBCid
	} else {
		md.AddFundsCid = DBCid(*src.AddFundsCid)
	}
	if src.PublishCid == nil {
		md.PublishCid = UndefDBCid
	} else {
		md.PublishCid = DBCid(*src.PublishCid)
	}

	if src.Ref != nil {
		md.Ref = DataRef{
			TransferType: src.Ref.TransferType,
			Root:         DBCid(src.Ref.Root),
			PieceSize:    src.Ref.PieceSize,
			RawBlockSize: src.Ref.RawBlockSize,
		}

		if src.Ref.PieceCid == nil {
			md.Ref.PieceCid = UndefDBCid
		} else {
			md.Ref.PieceCid = DBCid(*src.Ref.PieceCid)
		}
	}
	if src.TransferChannelID != nil {
		md.TransferChannelId = ChannelID{
			Initiator: src.TransferChannelID.Initiator.String(),
			Responder: src.TransferChannelID.Responder.String(),
			ID:        uint64(src.TransferChannelID.ID),
		}
	}

	return md
}

func toStorageDeal(src *storageDeal) (*types.MinerDeal, error) {
	var label vTypes.DealLabel
	var err error
	if utf8.ValidString(src.Label) {
		label, err = vTypes.NewLabelFromString(src.Label)
	} else {
		label, err = vTypes.NewLabelFromBytes([]byte(src.Label))
	}
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	if len(src.ID) != 0 {
		id, err = uuid.Parse(src.ID)
		if err != nil {
			return nil, err
		}
	}
	md := &types.MinerDeal{
		ID: id,
		ClientDealProposal: vTypes.ClientDealProposal{
			Proposal: vTypes.DealProposal{
				PieceCID:             src.PieceCID.cid(),
				PieceSize:            abi.PaddedPieceSize(src.PieceSize),
				VerifiedDeal:         src.VerifiedDeal,
				Client:               src.ClientDealProposal.Client.addr(),
				Provider:             src.ClientDealProposal.Provider.addr(),
				Label:                label,
				StartEpoch:           abi.ChainEpoch(src.StartEpoch),
				EndEpoch:             abi.ChainEpoch(src.EndEpoch),
				StoragePricePerEpoch: abi.TokenAmount(mtypes.SafeFromGo(src.StoragePricePerEpoch.Int)),
				ProviderCollateral:   abi.TokenAmount(mtypes.SafeFromGo(src.ProviderCollateral.Int)),
				ClientCollateral:     abi.TokenAmount(mtypes.SafeFromGo(src.ClientCollateral.Int)),
			},
			ClientSignature: acrypto.Signature{
				Type: src.ClientSignature.Type,
				Data: src.ClientSignature.Data,
			},
		},
		ProposalCid:   src.ProposalCid.cid(),
		AddFundsCid:   src.AddFundsCid.cidPtr(),
		PublishCid:    src.PublishCid.cidPtr(),
		State:         src.State,
		PayloadSize:   src.PayloadSize,
		PiecePath:     filestore.Path(src.PiecePath),
		MetadataPath:  filestore.Path(src.MetadataPath),
		PieceStatus:   types.PieceStatus(src.PieceStatus),
		SlashEpoch:    abi.ChainEpoch(src.SlashEpoch),
		FastRetrieval: src.FastRetrieval,
		Message:       src.Message,
		FundsReserved: abi.TokenAmount(mtypes.SafeFromGo(src.FundsReserved.Int)),
		Ref: &storagemarket.DataRef{
			TransferType: src.Ref.TransferType,
			Root:         src.Ref.Root.cid(),
			PieceCid:     src.Ref.PieceCid.cidPtr(),
			PieceSize:    src.Ref.PieceSize,
			RawBlockSize: src.Ref.RawBlockSize,
		},
		AvailableForRetrieval: src.AvailableForRetrieval,
		DealID:                abi.DealID(src.DealID),
		CreationTime:          typegen.CborTime(time.Unix(0, src.CreationTime).UTC()),
		SectorNumber:          abi.SectorNumber(src.SectorNumber),
		InboundCAR:            src.InboundCAR,
		Offset:                abi.PaddedPieceSize(src.Offset),
		TimeStamp:             src.Timestamp(),
	}

	if len(src.TransferChannelId.Initiator) > 0 {
		md.TransferChannelID = &datatransfer.ChannelID{}
		md.TransferChannelID.ID = datatransfer.TransferID(src.TransferChannelId.ID)
		md.TransferChannelID.Initiator, err = decodePeerId(src.TransferChannelId.Initiator)
		if err != nil {
			return nil, fmt.Errorf("decode tci_initiator: %s", err)
		}
		md.TransferChannelID.Responder, err = decodePeerId(src.TransferChannelId.Responder)
		if err != nil {
			return nil, fmt.Errorf("decode tci_responder: %s", err)
		}
	}

	// todo 导入的数据没有此字段
	md.Miner, err = decodePeerId(src.Miner)
	if err != nil {
		return nil, fmt.Errorf("decode miner_peer: %s", err)
	}

	md.Client, err = decodePeerId(src.Client)
	if err != nil {
		return nil, fmt.Errorf("decode client_peer: %s", err)
	}

	return md, nil
}

type storageDealRepo struct {
	*gorm.DB
}

var _ repo.StorageDealRepo = (*storageDealRepo)(nil)

func NewStorageDealRepo(db *gorm.DB) repo.StorageDealRepo {
	return &storageDealRepo{db}
}

func (sdr *storageDealRepo) CreateDeals(ctx context.Context, deals []*types.MinerDeal) error {
	storageDeals := make([]*storageDeal, 0, len(deals))
	for _, deal := range deals {
		storageDeal := fromStorageDeal(deal)
		storageDeal.TimeStampOrm.Refresh()
		storageDeals = append(storageDeals, storageDeal)
	}

	return sdr.WithContext(ctx).Clauses(
		clause.OnConflict{Columns: []clause.Column{{Name: "proposal_cid"}}, UpdateAll: true}).
		CreateInBatches(storageDeals, 100).Error
}

func (sdr *storageDealRepo) SaveDeal(ctx context.Context, storageDeal *types.MinerDeal) error {
	deal := fromStorageDeal(storageDeal)
	deal.TimeStampOrm.Refresh()

	return sdr.WithContext(ctx).Clauses(
		clause.OnConflict{Columns: []clause.Column{{Name: "proposal_cid"}}, UpdateAll: true}).
		Create(deal).Error
}

func (sdr *storageDealRepo) SaveDealWithStatus(ctx context.Context, storageDeal *types.MinerDeal, pieceStates []types.PieceStatus) error {
	deal := fromStorageDeal(storageDeal)
	deal.TimeStampOrm.Refresh()
	return sdr.WithContext(ctx).Where("id = ? and piece_status in ?", deal.ID, pieceStates).Save(deal).Error
}

func (sdr *storageDealRepo) GetDeal(ctx context.Context, proposalCid cid.Cid) (*types.MinerDeal, error) {
	var md storageDeal
	err := sdr.WithContext(ctx).Take(&md, "proposal_cid = ?", DBCid(proposalCid).String()).Error
	if err != nil {
		return nil, err
	}

	return toStorageDeal(&md)
}

func (sdr *storageDealRepo) GetDealByUUID(ctx context.Context, id uuid.UUID) (*types.MinerDeal, error) {
	var md storageDeal
	err := sdr.WithContext(ctx).Take(&md, "id = ?", id.String()).Error
	if err != nil {
		return nil, err
	}

	return toStorageDeal(&md)
}

func (sdr *storageDealRepo) GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*types.MinerDeal, error) {
	var md []storageDeal

	err := sdr.WithContext(ctx).Table((&storageDeal{}).TableName()).
		Find(&md, "cdp_provider = ?", DBAddress(miner).String()).
		Offset(pageIndex * pageSize).Limit(pageSize).Error
	if err != nil {
		return nil, err
	}

	deals := make([]*types.MinerDeal, len(md))

	for idx, deal := range md {
		if deals[idx], err = toStorageDeal(&deal); err != nil {
			return nil, fmt.Errorf("convert StorageDeal(%s) to a types.MinerDeal failed:%w",
				deal.ProposalCid, err)
		}
	}

	return deals, nil
}

func (sdr *storageDealRepo) GetDealsByPieceCidAndStatus(ctx context.Context, piececid cid.Cid, statues ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	var md []storageDeal

	err := sdr.WithContext(ctx).Table((&storageDeal{}).TableName()).
		Find(&md, "cdp_piece_cid = ? AND state in ?", DBCid(piececid).String(), statues).Error
	if err != nil {
		return nil, err
	}

	deals := make([]*types.MinerDeal, len(md))

	for idx, deal := range md {
		if deals[idx], err = toStorageDeal(&deal); err != nil {
			return nil, fmt.Errorf("convert StorageDeal(%s) to a types.MinerDeal failed:%w",
				deal.ProposalCid, err)
		}
	}

	return deals, nil
}

func (sdr *storageDealRepo) GetDealsByDataCidAndDealStatus(ctx context.Context, mAddr address.Address, dataCid cid.Cid, pieceStatuss []types.PieceStatus) ([]*types.MinerDeal, error) {
	var md []storageDeal

	query := sdr.WithContext(ctx).Table((&storageDeal{}).TableName()).Where("ref_root=?", DBCid(dataCid).String())
	if mAddr != address.Undef {
		query.Where("cdp_provider=?", DBAddress(mAddr).String())
	}
	if len(pieceStatuss) > 0 {
		query.Where("piece_status in ?", pieceStatuss)
	}
	err := query.Find(&md).Error
	if err != nil {
		return nil, err
	}

	deals := make([]*types.MinerDeal, len(md))

	for idx, deal := range md {
		if deals[idx], err = toStorageDeal(&deal); err != nil {
			return nil, fmt.Errorf("convert StorageDeal(%s) to a types.MinerDeal failed:%w",
				deal.ProposalCid, err)
		}
	}

	return deals, nil
}

func (sdr *storageDealRepo) GetDealByAddrAndStatus(ctx context.Context, mAddr address.Address, status ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	var md []storageDeal

	query := sdr.WithContext(ctx).Table((&storageDeal{}).TableName())
	if mAddr != address.Undef {
		query.Where("cdp_provider=?", DBAddress(mAddr).String())
	}
	if len(status) > 0 {
		query.Where("state in ?", status)
	}

	err := query.Find(&md).Error
	if err != nil {
		return nil, err
	}

	if len(md) == 0 {
		return nil, repo.ErrNotFound
	}

	deals := make([]*types.MinerDeal, len(md))

	for idx, deal := range md {
		if deals[idx], err = toStorageDeal(&deal); err != nil {
			return nil, fmt.Errorf("convert StorageDeal(%s) to a types.MinerDeal failed:%w",
				deal.ProposalCid, err)
		}
	}

	return deals, nil
}

func (sdr *storageDealRepo) UpdateDealStatus(ctx context.Context, proposalCid cid.Cid, status storagemarket.StorageDealStatus, pieceState types.PieceStatus) error {
	updateColumns := make(map[string]interface{})

	if status != storagemarket.StorageDealUnknown {
		updateColumns["state"] = status
	}

	if len(pieceState) != 0 {
		updateColumns["piece_status"] = pieceState
	}

	if len(updateColumns) == 0 {
		return nil
	}

	updateColumns["updated_at"] = time.Now().Unix()

	return sdr.WithContext(ctx).Model(storageDeal{}).Where("proposal_cid = ?", DBCid(proposalCid).String()).
		UpdateColumns(updateColumns).Error
}

func (sdr *storageDealRepo) ListDealByAddr(ctx context.Context, miner address.Address) ([]*types.MinerDeal, error) {
	var storageDeals []*storageDeal
	if err := sdr.Table(storageDealTableName).Find(&storageDeals, "cdp_provider = ?", DBAddress(miner).String()).Error; err != nil {
		return nil, err
	}
	return fromDbDeals(storageDeals)
}

func (sdr *storageDealRepo) ListDeal(ctx context.Context, params *types.StorageDealQueryParams) ([]*types.MinerDeal, error) {
	var storageDeals []*storageDeal
	discardFailedDeal := params.DiscardFailedDeal
	if discardFailedDeal && params.State != nil {
		state := *params.State
		discardFailedDeal = state != storagemarket.StorageDealFailing && state != storagemarket.StorageDealSlashed &&
			state != storagemarket.StorageDealExpired && state != storagemarket.StorageDealError
	}
	query := sdr.Table(storageDealTableName).Offset(params.Offset).Limit(params.Limit)
	if params.Asc {
		query.Order("created_at asc")
	} else {
		query.Order("created_at desc")
	}
	if !params.Miner.Empty() {
		query.Where("cdp_provider = ?", DBAddress(params.Miner).String())
	}
	if len(params.Client) > 0 {
		query.Where("client = ?", params.Client)
	}
	if params.State != nil {
		query.Where("state = ?", params.State)
	}
	if len(params.PieceCID) != 0 {
		query.Where("cdp_piece_cid = ?", params.PieceCID)
	}
	// attention: deal id can not be zero in mainnet, but it can be zero in testnet
	// is is a compromise for forward compatibility
	if params.DealID != 0 {
		query.Where("deal_id = ?", params.DealID)
	}
	if discardFailedDeal {
		states := []storagemarket.StorageDealStatus{storagemarket.StorageDealFailing,
			storagemarket.StorageDealExpired, storagemarket.StorageDealError, storagemarket.StorageDealSlashed}
		query.Where("state not in ?", states)
	}
	if err := query.Find(&storageDeals).Error; err != nil {
		return nil, err
	}

	return fromDbDeals(storageDeals)
}

func (sdr *storageDealRepo) GetPieceInfo(ctx context.Context, pieceCID cid.Cid) (*piecestore.PieceInfo, error) {
	var storageDeals []*storageDeal
	if err := sdr.Table(storageDealTableName).Find(&storageDeals, "cdp_piece_cid = ?", DBCid(pieceCID).String()).Error; err != nil {
		return nil, err
	}

	pieceInfo := piecestore.PieceInfo{
		PieceCID: pieceCID,
		Deals:    nil,
	}

	for _, dbDeal := range storageDeals {
		deal, err := toStorageDeal(dbDeal)
		if err != nil {
			return nil, err
		}
		pieceInfo.Deals = append(pieceInfo.Deals, piecestore.DealInfo{
			DealID:   deal.DealID,
			SectorID: deal.SectorNumber,
			Offset:   deal.Offset,
			Length:   deal.Proposal.PieceSize,
		},
		)
	}
	return &pieceInfo, nil
}

func (sdr *storageDealRepo) ListPieceInfoKeys(ctx context.Context) ([]cid.Cid, error) {
	var cidsStr []string
	var err error

	if err := sdr.DB.Table((&storageDeal{}).TableName()).Select("cdp_piece_cid").Scan(&cidsStr).Error; err != nil {
		return nil, err
	}

	cids := make([]cid.Cid, len(cidsStr))
	for idx, s := range cidsStr {
		cids[idx], err = cid.Decode(s)
		if err != nil {
			return nil, err
		}
	}

	return cids, nil
}

func (sdr *storageDealRepo) GetDealByDealID(ctx context.Context, mAddr address.Address, dealID abi.DealID) (*types.MinerDeal, error) {
	var dbDeal *storageDeal
	if err := sdr.WithContext(ctx).Table(storageDealTableName).Take(&dbDeal, "cdp_provider = ? and deal_id = ?", DBAddress(mAddr).String(), dealID).Error; err != nil {
		return nil, err
	}
	return toStorageDeal(dbDeal)
}

func (sdr *storageDealRepo) GetDealsByPieceStatusAndDealStatus(ctx context.Context, mAddr address.Address, pieceStatus types.PieceStatus, dealStatus ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	query := sdr.WithContext(ctx).Table(storageDealTableName).Where("piece_status = ?", pieceStatus)
	if len(dealStatus) > 0 {
		query.Where("state in ?", dealStatus)
	}
	if mAddr != address.Undef {
		query.Where("cdp_provider=?", DBAddress(mAddr).String())
	}

	var dbDeals []*storageDeal
	if err := query.Find(&dbDeals).Error; err != nil {
		return nil, err
	}

	return fromDbDeals(dbDeals)
}

func (sdr *storageDealRepo) GetPieceSize(ctx context.Context, pieceCID cid.Cid) (uint64, abi.PaddedPieceSize, error) {
	var deal storageDeal
	states := []storagemarket.StorageDealStatus{storagemarket.StorageDealFailing, storagemarket.StorageDealRejecting, storagemarket.StorageDealError}
	if err := sdr.WithContext(ctx).Table(storageDealTableName).Take(&deal, "cdp_piece_cid = ? and state not in ?",
		DBCid(pieceCID).String(), states).Error; err != nil {
		return 0, 0, err
	}

	return deal.PayloadSize, abi.PaddedPieceSize(deal.PieceSize), nil
}

func (sdr *storageDealRepo) GroupStorageDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[storagemarket.StorageDealStatus]int64, error) {
	query := sdr.WithContext(ctx).Table(storageDealTableName).Group("state").Select("state, count(1) as count")
	if mAddr != address.Undef {
		query.Where("cdp_provider = ?", DBAddress(mAddr).String())
	}

	var items []struct {
		State int
		Count int64
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}

	result := map[storagemarket.StorageDealStatus]int64{}
	for _, item := range items {
		result[storagemarket.StorageDealStatus(item.State)] = item.Count
	}
	return result, nil
}

func (sdr *storageDealRepo) CountDealByMiner(ctx context.Context,
	miner address.Address,
	state storagemarket.StorageDealStatus,
) (int64, error) {
	var count int64
	if err := sdr.WithContext(ctx).Model(&storageDeal{}).Where("cdp_provider = ? and state = ?",
		DBAddress(miner).String(), state).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func fromDbDeals(dbDeals []*storageDeal) ([]*types.MinerDeal, error) {
	results := make([]*types.MinerDeal, len(dbDeals))
	for index, dbDeal := range dbDeals {
		deal, err := toStorageDeal(dbDeal)
		if err != nil {
			return nil, err
		}
		results[index] = deal
	}
	return results, nil
}
