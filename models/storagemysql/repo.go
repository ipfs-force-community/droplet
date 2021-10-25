package storagemysql

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type TxRepo interface {
}

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
	CreateMinerDeal(minerDeal *types.MinerDeal) error
	GetDeal(proposalCid cid.Cid) (*types.MinerDeal, error)
	UpdateDeal(proposalCid cid.Cid, updateCols map[string]interface{}) error
	ListMinerDeal() ([]*types.MinerDeal, error)
}

type PaychMsgInfoRepo interface {
	GetMessage(mcid cid.Cid) (*types.MsgInfo, error)
	SaveMessage(info *types.MsgInfo) error
	SaveMessageResult(mcid cid.Cid, msgErr error) error
}

type PaychChannelInfoRepo interface {
	GetChannelByAddress(ch address.Address) (*types.ChannelInfo, error)
	GetChannelByChannelID(channelID string) (*types.ChannelInfo, error)
	GetChannelByMessageCid(mcid cid.Cid) (*types.ChannelInfo, error)
	ListChannel() ([]*types.ChannelInfo, error)
	SaveChannel(ci *types.ChannelInfo) error
	RemoveChannel(channelID string) error
}

type Repo interface {
	GetDb() *gorm.DB
	Transaction(func(txRepo TxRepo) error) error
	DbClose() error
	AutoMigrate() error

	FundRepo() FundRepo
	MinerParamsRepo() MinerParamsRepo
	MinerDealRepo() MinerDealRepo
	PaychMsgInfoRepo() PaychMsgInfoRepo
}

type repo struct {
	*gorm.DB
}

func (r repo) AutoMigrate() error {
	return r.DB.AutoMigrate(fundedAddressState{}, minerParams{}, minerDeal{}, msgInfo{}, channelInfo{})
}

func (r repo) GetDb() *gorm.DB {
	return r.DB
}

func (r repo) DbClose() error {
	return nil
}

func (r repo) Transaction(cb func(txRepo TxRepo) error) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		txRepo := &txRepo{DB: tx}
		return cb(txRepo)
	})
}

func (r repo) FundRepo() FundRepo {
	return newFundedAddressStateRepo(r.GetDb())
}

func (r repo) MinerParamsRepo() MinerParamsRepo {
	return newMinerParamsRepo(r.GetDb())
}

func (r repo) MinerDealRepo() MinerDealRepo {
	return newMinerDealRepo(r.GetDb())
}

func (r repo) PaychMsgInfoRepo() PaychMsgInfoRepo {
	return newMsgInfoRepo(r.GetDb())
}

func (r repo) PaychChannelInfo() PaychChannelInfoRepo {
	return newChannelInfoRepo(r.GetDb())
}

type txRepo struct {
	*gorm.DB
}

var _ Repo = (*repo)(nil)
var _ TxRepo = (*txRepo)(nil)
