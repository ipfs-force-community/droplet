package models

import (
	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

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
