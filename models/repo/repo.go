package repo

import (
	"errors"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

type FundRepo interface {
	GetFundedAddressState(addr address.Address) (*types.FundedAddressState, error)
	SaveFundedAddressState(fds *types.FundedAddressState) error
	ListFundedAddressState() ([]*types.FundedAddressState, error)
}

type StorageDealRepo interface {
	SaveDeal(StorageDeal *types.MinerDeal) error
	GetDeal(proposalCid cid.Cid) (*types.MinerDeal, error)
	GetDeals(mAddr address.Address, pageIndex, pageSize int) ([]*types.MinerDeal, error)
	GetDealsByPieceStatus(mAddr address.Address, pieceStatus string) ([]*types.MinerDeal, error)
	GetDealByDealID(mAddr address.Address, dealID abi.DealID) (*types.MinerDeal, error)
	ListDeal(mAddr address.Address) ([]*types.MinerDeal, error)
	GetPieceInfo(pieceCID cid.Cid) (*piecestore.PieceInfo, error)
	ListPieceInfoKeys() ([]cid.Cid, error)
}

type IRetrievalDealRepo interface {
	SaveDeal(deal *retrievalmarket.ProviderDealState) error
	GetDeal(peer.ID, retrievalmarket.DealID) (*retrievalmarket.ProviderDealState, error)
	HasDeal(peer.ID, retrievalmarket.DealID) (bool, error)
	ListDeals(pageIndex, pageSize int) ([]*retrievalmarket.ProviderDealState, error)
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

type ICidInfoRepo interface {
	// use StorageDealRepo.SaveDeal with fields:
	// 	Offset abi.PaddedPieceSize
	//	Length abi.PaddedPieceSize
	// TODO: add a 'AddDealForPiece' interface in StorageDealRepo ?
	// AddDealForPiece(pieceCID cid.Cid, dealInfo piecestore.DealInfo) error
	AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error
	GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error)
	ListCidInfoKeys() ([]cid.Cid, error)
	Close() error
	// ListPieceInfoKeys() ([]cid.Cid, error)
	// GetPieceInfoFromCid(ctx context.Context, payloadCID, pieceCID cid.Cid) (piecestore.PieceInfo, bool, error)
}

type Repo interface {
	FundRepo() FundRepo
	StorageDealRepo() StorageDealRepo
	PaychMsgInfoRepo() PaychMsgInfoRepo
	PaychChannelInfoRepo() PaychChannelInfoRepo
	StorageAskRepo() IStorageAskRepo
	RetrievalAskRepo() IRetrievalAskRepo
	CidInfoRepo() ICidInfoRepo
	RetrievalDealRepo() IRetrievalDealRepo
}

var ErrNotFound = errors.New("not found")
