package v230

import (
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/models/badger/statestore"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	peer "github.com/libp2p/go-libp2p/core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
)

type MinerDeal struct {
	types.ClientDealProposal
	ProposalCid           cid.Cid
	AddFundsCid           *cid.Cid
	PublishCid            *cid.Cid
	Miner                 peer.ID
	Client                peer.ID
	State                 storagemarket.StorageDealStatus
	PiecePath             filestore.Path
	PayloadSize           uint64
	MetadataPath          filestore.Path
	SlashEpoch            abi.ChainEpoch
	FastRetrieval         bool
	Message               string
	FundsReserved         abi.TokenAmount
	Ref                   *storagemarket.DataRef
	AvailableForRetrieval bool

	DealID       abi.DealID
	CreationTime cbg.CborTime

	TransferChannelID *datatransfer.ChannelID
	SectorNumber      abi.SectorNumber

	Offset      abi.PaddedPieceSize
	PieceStatus market.PieceStatus

	InboundCAR string

	market.TimeStamp
}

func (t *MinerDeal) KeyWithNamespace() datastore.Key {
	return datastore.KeyWithNamespaces([]string{
		"/storage/provider/deals/1",
		statestore.ToKey(t.ProposalCid).String(),
	})
}
