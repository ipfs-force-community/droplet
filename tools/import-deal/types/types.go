package types

import (
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus-messager/models/mtypes"

	"github.com/filecoin-project/venus-market/v2/models/mysql"
)

const (
	TTFromForce = "import"
)

const DefaultPeerID = "12D3KooWQztpkQoRR1k3xmRowa3pwA8qDg9yKyiqQLGr6Y4tWkq5"

type ForceDeal struct {
	ID               uint64    `gorm:"primary_key;column:id;type:bigint(20) unsigned AUTO_INCREMENT;not null" json:"id"`
	Dealid           uint64    `gorm:"column:dealid;unique_index:uq_dealid_ttype;type:bigint(20) unsigned" json:"dealid"`
	Sectorid         uint64    `gorm:"column:sectorid;type:bigint(20) unsigned;index:idx_sector_id" json:"sectorid"`
	Offset           uint64    `gorm:"column:offset;type:bigint(20) unsigned" json:"offset"`
	Unpadsize        uint64    `gorm:"column:unpadsize;type:bigint(20) unsigned" json:"unpadsize"`
	Filesize         uint64    `gorm:"column:filesize;type:bigint(20) unsigned" json:"filesize"`
	Finish           bool      `gorm:"column:finish;type:tinyint(3) unsigned" json:"finish"`
	Start            uint64    `gorm:"column:start;type:bigint(20) unsigned" json:"start"`
	End              uint64    `gorm:"column:end;type:bigint(20) unsigned" json:"end"`
	Fetch            bool      `gorm:"column:fetch;type:tinyint(3) unsigned" json:"fetch"`
	Piececid         string    `gorm:"column:piececid;type:varchar(255)" json:"piececid"`
	Rootcid          string    `gorm:"column:rootcid;type:varchar(255)" json:"rootcid"`
	Client           string    `gorm:"column:client;type:varchar(255)" json:"client"`
	Provider         string    `gorm:"column:provider;type:varchar(255)" json:"provider"`
	ClientCollateral uint64    `gorm:"column:ccollateral;type:bigint(20) unsigned" json:"ccollateral"`
	Price            uint64    `gorm:"column:price;type:bigint(20) unsigned" json:"price"`
	Createtime       time.Time `gorm:"column:createtime;type:timestamp" json:"createtime"`
	Proposalcid      string    `gorm:"column:proposalcid;type:varchar(255)" json:"proposalcid"`
	Peerid           string    `gorm:"column:peerid;type:varchar(255)" json:"peerid"`
	Isabandon        bool      `gorm:"column:isabandon;not null;type:tinyint(3) unsigned" json:"isabandon"`
}

func convertBigInt(v uint64) mtypes.Int {
	return mtypes.NewInt(int64(v))
}

func (fd *ForceDeal) ToDeal() *Deal {
	pCid, _ := cid.Decode(fd.Piececid)
	rootCid, _ := cid.Decode(fd.Rootcid)
	client, _ := address.NewFromString(fd.Client)
	provider, _ := address.NewFromString(fd.Provider)
	proposalCid, _ := cid.Decode(fd.Proposalcid)

	md := &Deal{
		ClientDealProposal: mysql.ClientDealProposal{
			PieceCID:             mysql.DBCid(pCid),
			VerifiedDeal:         true,
			Client:               mysql.DBAddress(client),
			Provider:             mysql.DBAddress(provider),
			StartEpoch:           int64(fd.Start),
			EndEpoch:             int64(fd.End),
			StoragePricePerEpoch: convertBigInt(fd.Price),
			ClientCollateral:     convertBigInt(fd.ClientCollateral),
		},
		ProposalCid: mysql.DBCid(proposalCid),
		Miner:       DefaultPeerID, // todo 反序列化时需要能解析
		Client:      fd.Peerid,
		State:       storagemarket.StorageDealActive,
		PayloadSize: int64(fd.Filesize),
		Ref: mysql.DataRef{
			TransferType: TTFromForce,
			Root:         mysql.DBCid(rootCid),

			PieceCid: mysql.UndefDBCid,
		},
		DealID:       fd.Dealid,
		CreationTime: fd.Createtime.UnixNano(),
		SectorNumber: fd.Sectorid,

		Offset:      fd.Offset,
		PieceStatus: "Proving",
	}

	return md
}

type Deal struct {
	mysql.ClientDealProposal `gorm:"embedded;embeddedPrefix:cdp_"`

	ProposalCid mysql.DBCid `gorm:"column:proposal_cid;type:varchar(256);primary_key"`
	AddFundsCid mysql.DBCid `gorm:"column:add_funds_cid;type:varchar(256);"`
	PublishCid  mysql.DBCid `gorm:"column:publish_cid;type:varchar(256);"`
	Miner       string      `gorm:"column:miner_peer;type:varchar(128);"`
	Client      string      `gorm:"column:client_peer;type:varchar(128);"`
	State       uint64      `gorm:"column:state;type:bigint unsigned;index"`

	PayloadSize           int64         `gorm:"column:payload_size;type:bigint;"`
	PiecePath             string        `gorm:"column:piece_path;type:varchar(256);"`
	MetadataPath          string        `gorm:"column:metadata_path;type:varchar(256);"`
	SlashEpoch            int64         `gorm:"column:slash_epoch;type:bigint;"`
	FastRetrieval         bool          `gorm:"column:fast_retrieval;"`
	Message               string        `gorm:"column:message;type:varchar(512);"`
	FundsReserved         mtypes.Int    `gorm:"column:funds_reserved;type:varchar(256);"`
	Ref                   mysql.DataRef `gorm:"embedded;embeddedPrefix:ref_"`
	AvailableForRetrieval bool          `gorm:"column:available_for_retrieval;"`

	DealID       uint64 `gorm:"column:deal_id;type:bigint unsigned;index"`
	CreationTime int64  `gorm:"column:creation_time;type:bigint;"`

	TransferChannelId mysql.ChannelID `gorm:"embedded;embeddedPrefix:tci_"`
	SectorNumber      uint64          `gorm:"column:sector_number;type:bigint unsigned;"`

	InboundCAR string `gorm:"column:addr;type:varchar(256);"`

	Offset      uint64 `gorm:"column:offset;type:bigint"`
	Length      uint64 `gorm:"column:length;type:bigint"`
	PieceStatus string `gorm:"column:piece_status;type:varchar(128);index"`

	mysql.TimeStampOrm
}

func (Deal) TableName() string {
	return "storage_deals"
}
