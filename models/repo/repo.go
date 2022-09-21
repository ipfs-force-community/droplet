package repo

import (
	"context"
	"errors"

	"github.com/ipfs/go-datastore"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

type FundRepo interface {
	GetFundedAddressState(ctx context.Context, addr address.Address) (*types.FundedAddressState, error)
	SaveFundedAddressState(ctx context.Context, fds *types.FundedAddressState) error
	ListFundedAddressState(ctx context.Context) ([]*types.FundedAddressState, error)
}

type StorageDealRepo interface {
	SaveDeal(ctx context.Context, StorageDeal *types.MinerDeal) error
	UpdateDealStatus(ctx context.Context, proposalCid cid.Cid, status storagemarket.StorageDealStatus, pieceState types.PieceStatus) error

	GetDeal(ctx context.Context, proposalCid cid.Cid) (*types.MinerDeal, error)
	GetDealByDealID(ctx context.Context, mAddr address.Address, dealID abi.DealID) (*types.MinerDeal, error)

	//todo rename Getxxx to Listxxx if return deals list
	GetDeals(ctx context.Context, mAddr address.Address, pageIndex, pageSize int) ([]*types.MinerDeal, error)
	//GetDealsByPieceStatusAndDealStatus list deals by providor, piece status and deal status, but if addr is Undef, only filter by piece status
	GetDealsByPieceStatusAndDealStatus(ctx context.Context, mAddr address.Address, pieceStatus types.PieceStatus, dealStatus ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error)
	//GetDealsByDataCidAndDealStatus query deals from address data cid and deal status, if mAddr equal undef wont filter by address
	GetDealsByDataCidAndDealStatus(ctx context.Context, mAddr address.Address, dataCid cid.Cid, pieceStatuss []types.PieceStatus) ([]*types.MinerDeal, error)
	GetDealsByPieceCidAndStatus(ctx context.Context, piececid cid.Cid, statues ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error)
	GetDealByAddrAndStatus(ctx context.Context, addr address.Address, status ...storagemarket.StorageDealStatus) ([]*types.MinerDeal, error)
	ListDealByAddr(ctx context.Context, mAddr address.Address) ([]*types.MinerDeal, error)
	ListDeal(ctx context.Context) ([]*types.MinerDeal, error)
	GroupStorageDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[storagemarket.StorageDealStatus]int64, error)

	GetPieceInfo(ctx context.Context, pieceCID cid.Cid) (*piecestore.PieceInfo, error)
	GetPieceSize(ctx context.Context, pieceCID cid.Cid) (uint64, abi.PaddedPieceSize, error)
	ListPieceInfoKeys(ctx context.Context) ([]cid.Cid, error)
}

type IRetrievalDealRepo interface {
	SaveDeal(context.Context, *types.ProviderDealState) error
	GetDeal(context.Context, peer.ID, retrievalmarket.DealID) (*types.ProviderDealState, error)
	GetDealByTransferId(context.Context, datatransfer.ChannelID) (*types.ProviderDealState, error)
	HasDeal(context.Context, peer.ID, retrievalmarket.DealID) (bool, error)
	//ListDeals pageIndex index from 1
	ListDeals(ctx context.Context, pageIndex int, pageSize int) ([]*types.ProviderDealState, error)
	GroupRetrievalDealNumberByStatus(ctx context.Context, mAddr address.Address) (map[retrievalmarket.DealStatus]int64, error)
}

type PaychMsgInfoRepo interface {
	GetMessage(ctx context.Context, mcid cid.Cid) (*types.MsgInfo, error)
	SaveMessage(ctx context.Context, info *types.MsgInfo) error
	SaveMessageResult(ctx context.Context, mcid cid.Cid, msgErr error) error
}

type PaychChannelInfoRepo interface {
	CreateChannel(ctx context.Context, from address.Address, to address.Address, createMsgCid cid.Cid, amt fbig.Int) (*types.ChannelInfo, error)
	GetChannelByAddress(ctx context.Context, ch address.Address) (*types.ChannelInfo, error)
	GetChannelByChannelID(ctx context.Context, channelID string) (*types.ChannelInfo, error)
	GetChannelByMessageCid(ctx context.Context, mcid cid.Cid) (*types.ChannelInfo, error)
	WithPendingAddFunds(ctx context.Context) ([]*types.ChannelInfo, error)
	OutboundActiveByFromTo(ctx context.Context, from address.Address, to address.Address) (*types.ChannelInfo, error)
	ListChannel(ctx context.Context) ([]address.Address, error)
	SaveChannel(ctx context.Context, ci *types.ChannelInfo) error
	RemoveChannel(ctx context.Context, channelID string) error
}

type IStorageAskRepo interface {
	ListAsk(ctx context.Context) ([]*types.SignedStorageAsk, error)
	GetAsk(ctx context.Context, miner address.Address) (*types.SignedStorageAsk, error)
	SetAsk(ctx context.Context, ask *types.SignedStorageAsk) error
}

type IRetrievalAskRepo interface {
	ListAsk(ctx context.Context) ([]*types.RetrievalAsk, error)
	GetAsk(ctx context.Context, addr address.Address) (*types.RetrievalAsk, error)
	SetAsk(ctx context.Context, ask *types.RetrievalAsk) error
}

type ICidInfoRepo interface {
	AddPieceBlockLocations(ctx context.Context, pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error
	GetCIDInfo(ctx context.Context, payloadCID cid.Cid) (piecestore.CIDInfo, error)
	ListCidInfoKeys(ctx context.Context) ([]cid.Cid, error)
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
	Close() error
	Migrate() error
	Transaction(func(txRepo TxRepo) error) error
}

type TxRepo interface {
	StorageDealRepo() StorageDealRepo
}

var ErrNotFound = errors.New("record not found")

func UniformNotFoundErrors() {
	mongo.ErrNoDocuments = ErrNotFound
	datastore.ErrNotFound = ErrNotFound
	gorm.ErrRecordNotFound = ErrNotFound
}

func init() {
	UniformNotFoundErrors()
}
