package itf

import (
	"errors"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs/go-cid"
)

type FundRepo interface {
	GetFundedAddressState(addr address.Address) (*types.FundedAddressState, error)
	SaveFundedAddressState(fds *types.FundedAddressState) error
	ListFundedAddressState() ([]*types.FundedAddressState, error)
}

type MinerDealRepo interface {
	SaveMinerDeal(minerDeal *storagemarket.MinerDeal) error
	GetMinerDeal(proposalCid cid.Cid) (*storagemarket.MinerDeal, error)
	ListMinerDeal() ([]*storagemarket.MinerDeal, error)
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

type IStorageAskRepo interface {
	GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error)
	SetAsk(ask *storagemarket.SignedStorageAsk) error
	Close() error
}

type IRetrievalAskRepo interface {
	GetAsk(addr address.Address) (*retrievalmarket.Ask, error)
	SetAsk(addr address.Address, ask *retrievalmarket.Ask) error
	Close() error
}

type Repo interface {
	FundRepo() FundRepo
	MinerDealRepo() MinerDealRepo
	PaychMsgInfoRepo() PaychMsgInfoRepo
	PaychChannelInfoRepo() PaychChannelInfoRepo
	StorageAskRepo() IStorageAskRepo
	RetrievalAskRepo() IRetrievalAskRepo
}

var ErrNotFound = errors.New("not found")