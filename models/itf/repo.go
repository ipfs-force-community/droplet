package itf

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/venus-market/types"
)

type FundRepo interface {
	GetFundedAddressState(addr address.Address) (*types.FundedAddressState, error)
	SaveFundedAddressState(fds *types.FundedAddressState) error
	ListFundedAddressState() ([]*types.FundedAddressState, error)
}

type MinerParamsRepo interface {
	CreateMinerParams(*types.MinerParams) error
	GetMinerParams(miner address.Address) (*types.MinerParams, error)
	UpdateMinerParams(miner address.Address, updateCols map[string]interface{}) error
	ListMinerParams() ([]*types.MinerParams, error)
}

type MinerDealRepo interface {
	SaveMinerDeal(minerDeal *types.MinerDeal) error
	GetMinerDeal(proposalCid cid.Cid) (*types.MinerDeal, error)
	UpdateMinerDeal(proposalCid cid.Cid, updateCols map[string]interface{}) error
	ListMinerDeal() ([]*types.MinerDeal, error)
}

type PaychMsgInfoRepo interface {
	GetMessage(mcid cid.Cid) (*types.MsgInfo, error)
	SaveMessage(info *types.MsgInfo) error
	SaveMessageResult(mcid cid.Cid, msgErr error) error
}

type PaychChannelInfoRepo interface {
	CreateChannel(from address.Address, to address.Address, createMsgCid cid.Cid, amt fbig.Int) (*types.ChannelInfo, error)
	GetChannelByAddress(ch address.Address) (*types.ChannelInfo, error)
	GetChannelByChannelID(channelID string) (*types.ChannelInfo, error)
	GetChannelByMessageCid(mcid cid.Cid) (*types.ChannelInfo, error)
	WithPendingAddFunds() ([]*types.ChannelInfo, error)
	OutboundActiveByFromTo(from address.Address, to address.Address) (*types.ChannelInfo, error)
	ListChannel() ([]address.Address, error)
	SaveChannel(ci *types.ChannelInfo) error
	RemoveChannel(channelID string) error
}

type Repo interface {
	FundRepo() FundRepo
	MinerParamsRepo() MinerParamsRepo
	MinerDealRepo() MinerDealRepo
	PaychMsgInfoRepo() PaychMsgInfoRepo
}

type StorageAskRepo interface {
	GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error)
	SetAsk(miner address.Address, ask *storagemarket.SignedStorageAsk) error
	Close() error
}

// /metadata
type MetadataDS datastore.Batching

// /metadata/fundmgr
type FundMgrDS datastore.Batching

// /metadata/storagemarket
type PieceMetaDs datastore.Batching

//  /metadata/storagemarket/cid-infos
type CIDInfoDS datastore.Batching

//  /metadata/storagemarket/pieces
type PieceInfoDS datastore.Batching

// /metadata/retrievals/provider
type RetrievalProviderDS datastore.Batching

// /metadata/retrievals/provider/retrieval-ask
type RetrievalAskDS datastore.Batching //key = latest

// /metadata/datatransfer/provider/transfers
type DagTransferDS datastore.Batching

// /metadata/deals/provider
type ProviderDealDS datastore.Batching

//   /metadata/deals/provider/storage-ask
type StorageAskDS datastore.Batching //key = latest

// /metadata/paych/
type PayChanDS datastore.Batching

//*********************************client
// /metadata/deals/client
type ClientDatastore datastore.Batching

// /metadata/deals/local
type ClientDealsDS datastore.Batching

// /metadata/retrievals/client
type RetrievalClientDS datastore.Batching

// /metadata/client
type ImportClientDS datastore.Batching

// /metadata/datatransfer/client/transfers
type ClientTransferDS datastore.Batching
