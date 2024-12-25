package impl

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"go.uber.org/fx"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/pkg/errors"

	"github.com/ipfs-force-community/sophon-auth/jwtclient"

	clients2 "github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/indexprovider"
	"github.com/ipfs-force-community/droplet/v2/minermgr"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/paychmgr"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs-force-community/droplet/v2/retrievalprovider"
	"github.com/ipfs-force-community/droplet/v2/storageprovider"
	"github.com/ipfs-force-community/droplet/v2/version"

	"github.com/filecoin-project/venus/pkg/constants"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	gatewayAPIV2 "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	marketAPI "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	gatewayTypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

var (
	_   marketAPI.IMarket = (*MarketNodeImpl)(nil)
	log                   = logging.Logger("market_api")
)

type MarketNodeImpl struct {
	fx.In

	FundAPI
	gatewayAPIV2.IMarketServiceProvider

	FullNode          v1api.FullNode
	Host              host.Host
	StorageProvider   storageprovider.StorageProvider
	RetrievalProvider retrievalprovider.IRetrievalProvider
	DataTransfer      network.ProviderDataTransfer
	DealPublisher     *storageprovider.DealPublisher
	DealAssigner      storageprovider.DealAssiger
	IndexProviderMgr  *indexprovider.IndexProviderMgr

	DirectDealProvider *storageprovider.DirectDealProvider

	AuthClient jwtclient.IAuthClient

	Messager                                    clients2.IMixMessage
	StorageAsk                                  storageprovider.IStorageAsk
	DAGStore                                    *dagstore.DAGStore
	DAGStoreWrapper                             stores.DAGStoreWrapper
	PieceStorageMgr                             *piecestorage.PieceStorageManager
	UserMgr                                     minermgr.IMinerMgr
	PaychAPI                                    *paychmgr.PaychAPI
	Repo                                        repo.Repo
	Config                                      *config.MarketConfig
	ConsiderOnlineStorageDealsConfigFunc        config.ConsiderOnlineStorageDealsConfigFunc
	SetConsiderOnlineStorageDealsConfigFunc     config.SetConsiderOnlineStorageDealsConfigFunc
	ConsiderOnlineRetrievalDealsConfigFunc      config.ConsiderOnlineRetrievalDealsConfigFunc
	SetConsiderOnlineRetrievalDealsConfigFunc   config.SetConsiderOnlineRetrievalDealsConfigFunc
	StorageDealPieceCidBlocklistConfigFunc      config.StorageDealPieceCidBlocklistConfigFunc
	SetStorageDealPieceCidBlocklistConfigFunc   config.SetStorageDealPieceCidBlocklistConfigFunc
	ConsiderOfflineStorageDealsConfigFunc       config.ConsiderOfflineStorageDealsConfigFunc
	SetConsiderOfflineStorageDealsConfigFunc    config.SetConsiderOfflineStorageDealsConfigFunc
	ConsiderOfflineRetrievalDealsConfigFunc     config.ConsiderOfflineRetrievalDealsConfigFunc
	SetConsiderOfflineRetrievalDealsConfigFunc  config.SetConsiderOfflineRetrievalDealsConfigFunc
	ConsiderVerifiedStorageDealsConfigFunc      config.ConsiderVerifiedStorageDealsConfigFunc
	SetConsiderVerifiedStorageDealsConfigFunc   config.SetConsiderVerifiedStorageDealsConfigFunc
	ConsiderUnverifiedStorageDealsConfigFunc    config.ConsiderUnverifiedStorageDealsConfigFunc
	SetConsiderUnverifiedStorageDealsConfigFunc config.SetConsiderUnverifiedStorageDealsConfigFunc

	GetExpectedSealDurationFunc config.GetExpectedSealDurationFunc
	SetExpectedSealDurationFunc config.SetExpectedSealDurationFunc

	GetMaxDealStartDelayFunc config.GetMaxDealStartDelayFunc
	SetMaxDealStartDelayFunc config.SetMaxDealStartDelayFunc

	TransferPathFunc    config.TransferPathFunc
	SetTransferPathFunc config.SetTransferPathFunc

	PublishMsgPeriodConfigFunc             config.PublishMsgPeriodConfigFunc
	SetPublishMsgPeriodConfigFunc          config.SetPublishMsgPeriodConfigFunc
	MaxDealsPerPublishMsgFunc              config.MaxDealsPerPublishMsgFunc
	SetMaxDealsPerPublishMsgFunc           config.SetMaxDealsPerPublishMsgFunc
	MaxProviderCollateralMultiplierFunc    config.MaxProviderCollateralMultiplierFunc
	SetMaxProviderCollateralMultiplierFunc config.SetMaxProviderCollateralMultiplierFunc

	MaxPublishDealsFeeFunc        config.MaxPublishDealsFeeFunc
	SetMaxPublishDealsFeeFunc     config.SetMaxPublishDealsFeeFunc
	MaxMarketBalanceAddFeeFunc    config.MaxMarketBalanceAddFeeFunc
	SetMaxMarketBalanceAddFeeFunc config.SetMaxMarketBalanceAddFeeFunc
}

func (m *MarketNodeImpl) ResponseMarketEvent(ctx context.Context, resp *gatewayTypes.ResponseEvent) error {
	return m.IMarketServiceProvider.ResponseMarketEvent(ctx, resp)
}

func (m *MarketNodeImpl) ListenMarketEvent(ctx context.Context, policy *gatewayTypes.MarketRegisterPolicy) (<-chan *gatewayTypes.RequestEvent, error) {
	return m.IMarketServiceProvider.ListenMarketEvent(ctx, policy)
}

func (m *MarketNodeImpl) ActorUpsert(ctx context.Context, user types.User) (bool, error) {
	err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, user.Addr)
	if err != nil {
		return false, err
	}

	bAdd, err := m.UserMgr.ActorUpsert(ctx, user)
	if err != nil {
		return false, err
	}

	if bAdd {
		m.Config.Miners = append(m.Config.Miners, &config.MinerConfig{
			Addr:    config.Address(user.Addr),
			Account: user.Account,
		})
	} else {
		for idx := range m.Config.Miners {
			if m.Config.Miners[idx].Addr == config.Address(user.Addr) {
				m.Config.Miners[idx].Account = user.Account
				break
			}
		}
	}
	err = config.SaveConfig(m.Config)
	if err != nil {
		return bAdd, err
	}

	return bAdd, nil
}

func (m *MarketNodeImpl) ActorDelete(ctx context.Context, mAddr address.Address) error {
	err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr)
	if err != nil {
		return err
	}

	err = m.UserMgr.ActorDelete(ctx, mAddr)
	if err != nil {
		return err
	}

	for idx := range m.Config.Miners {
		if m.Config.Miners[idx].Addr == config.Address(mAddr) {
			m.Config.Miners = append(m.Config.Miners[:idx], m.Config.Miners[idx+1:]...)
			break
		}
	}

	err = config.SaveConfig(m.Config)
	if err != nil {
		return err
	}

	return nil
}

func (m *MarketNodeImpl) ActorList(ctx context.Context) ([]types.User, error) {
	actors, err := m.UserMgr.ActorList(ctx)
	if err != nil {
		return nil, err
	}
	ret := make([]types.User, 0)
	for _, actor := range actors {
		if err := jwtclient.CheckPermissionByName(ctx, actor.Account); err == nil {
			ret = append(ret, actor)
		}
	}
	return ret, nil
}

func (m *MarketNodeImpl) ActorExist(ctx context.Context, addr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
		return false, err
	}
	return m.UserMgr.Has(ctx, addr), nil
}

func (m *MarketNodeImpl) ActorSectorSize(ctx context.Context, addr address.Address) (abi.SectorSize, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
		return 0, err
	}
	if bHas := m.UserMgr.Has(ctx, addr); bHas {
		minerInfo, err := m.FullNode.StateMinerInfo(ctx, addr, vTypes.EmptyTSK)
		if err != nil {
			return 0, err
		}

		return minerInfo.SectorSize, nil
	}

	return 0, errors.New("not found")
}

func (m *MarketNodeImpl) MarketImportDealData(ctx context.Context, propCid cid.Cid, path string) error {
	fi, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fi.Close() //nolint:errcheck

	ref := types.ImportDataRef{
		ProposalCID: propCid,
		File:        path,
	}

	res, err := m.StorageProvider.ImportDataForDeals(ctx, []*types.ImportDataRef{&ref}, false)
	if err != nil {
		return err
	}
	if len(res[0].Message) > 0 {
		return fmt.Errorf(res[0].Message)
	}

	return nil
}

func (m *MarketNodeImpl) MarketImportPublishedDeal(ctx context.Context, deal types.MinerDeal) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, deal.Proposal.Provider); err != nil {
		return err
	}
	return m.StorageProvider.ImportPublishedDeal(ctx, deal)
}

func (m *MarketNodeImpl) MarketListDeals(ctx context.Context, addrs []address.Address) ([]*vTypes.MarketDeal, error) {
	for _, addr := range addrs {
		if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
			return nil, errors.Errorf("check permission of miner %s failed: %s", addr, err)
		}
	}
	return m.listDeals(ctx, addrs)
}

func (m *MarketNodeImpl) MarketGetDeal(ctx context.Context, dealPropCid cid.Cid) (*types.MinerDeal, error) {
	deal, err := m.Repo.StorageDealRepo().GetDeal(ctx, dealPropCid)
	if err != nil {
		return nil, err
	}
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, deal.Proposal.Provider); err != nil {
		return nil, err
	}

	return deal, nil
}

// MarketListRetrievalDeals todo add user isolate when is available to get miner from retrieve deal
// 检索订单没法按 `miner address` 过滤
func (m *MarketNodeImpl) MarketListRetrievalDeals(ctx context.Context, params *types.RetrievalDealQueryParams) ([]types.ProviderDealState, error) {
	if params == nil {
		return nil, fmt.Errorf("params is empty")
	}

	var out []types.ProviderDealState
	deals, err := m.RetrievalProvider.ListDeals(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, deal := range deals {
		if deal.ChannelID != nil {
			if deal.ChannelID.Initiator == "" || deal.ChannelID.Responder == "" {
				deal.ChannelID = nil // don't try to push unparsable peer IDs over jsonrpc
			}
		}
		out = append(out, *deal)
	}
	return out, nil
}

func (m *MarketNodeImpl) MarketGetRetrievalDeal(ctx context.Context, receiver peer.ID, dealID uint64) (*types.ProviderDealState, error) {
	deal, err := m.Repo.RetrievalDealRepo().GetDeal(ctx, receiver, retrievalmarket.DealID(dealID))
	if err != nil {
		return nil, err
	}

	return deal, nil
}

func (m *MarketNodeImpl) MarketGetDealUpdates(ctx context.Context) (<-chan types.MinerDeal, error) {
	results := make(chan types.MinerDeal)
	unsub := m.StorageProvider.SubscribeToEvents(func(evt storagemarket.ProviderEvent, deal *types.MinerDeal) {
		select {
		case results <- *deal:
		case <-ctx.Done():
		}
	})
	go func() {
		<-ctx.Done()
		unsub()
		close(results)
	}()
	return results, nil
}

func (m *MarketNodeImpl) MarketListIncompleteDeals(ctx context.Context, params *types.StorageDealQueryParams) ([]types.MinerDeal, error) {
	if params == nil {
		return nil, fmt.Errorf("params is nil")
	}

	var err error
	if !params.Miner.Empty() {
		if err = jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, params.Miner); err != nil {
			return nil, err
		}
	}

	deals, err := m.Repo.StorageDealRepo().ListDeal(ctx, params)
	if err != nil {
		return nil, err
	}

	resDeals := make([]types.MinerDeal, len(deals))
	for idx, deal := range deals {
		resDeals[idx] = *deal
	}

	return resDeals, nil
}

func (m *MarketNodeImpl) UpdateStorageDealStatus(ctx context.Context, dealProposal cid.Cid, state storagemarket.StorageDealStatus, pieceState types.PieceStatus) error {
	deal, err := m.Repo.StorageDealRepo().GetDeal(ctx, dealProposal)
	if err != nil {
		return err
	}
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, deal.Proposal.Provider); err != nil {
		return err
	}
	return m.Repo.StorageDealRepo().UpdateDealStatus(ctx, dealProposal, state, pieceState)
}

func (m *MarketNodeImpl) MarketSetAsk(ctx context.Context, mAddr address.Address, price vTypes.BigInt, verifiedPrice vTypes.BigInt, duration abi.ChainEpoch, minPieceSize abi.PaddedPieceSize, maxPieceSize abi.PaddedPieceSize) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	options := []storagemarket.StorageAskOption{
		storagemarket.MinPieceSize(minPieceSize),
		storagemarket.MaxPieceSize(maxPieceSize),
	}

	return m.StorageAsk.SetAsk(ctx, mAddr, price, verifiedPrice, duration, options...)
}

func (m *MarketNodeImpl) MarketListStorageAsk(ctx context.Context) ([]*types.SignedStorageAsk, error) {
	return m.StorageAsk.ListAsk(ctx)
}

func (m *MarketNodeImpl) MarketGetAsk(ctx context.Context, mAddr address.Address) (*types.SignedStorageAsk, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return nil, err
	}
	return m.StorageAsk.GetAsk(ctx, mAddr)
}

func (m *MarketNodeImpl) MarketSetRetrievalAsk(ctx context.Context, mAddr address.Address, ask *retrievalmarket.Ask) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.Repo.RetrievalAskRepo().SetAsk(ctx, &types.RetrievalAsk{
		Miner:                   mAddr,
		PricePerByte:            ask.PricePerByte,
		UnsealPrice:             ask.UnsealPrice,
		PaymentInterval:         ask.PaymentInterval,
		PaymentIntervalIncrease: ask.PaymentIntervalIncrease,
	})
}

func (m *MarketNodeImpl) MarketListRetrievalAsk(ctx context.Context) ([]*types.RetrievalAsk, error) {
	return m.Repo.RetrievalAskRepo().ListAsk(ctx)
}

func (m *MarketNodeImpl) MarketGetRetrievalAsk(ctx context.Context, mAddr address.Address) (*retrievalmarket.Ask, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return nil, err
	}
	ask, err := m.Repo.RetrievalAskRepo().GetAsk(ctx, mAddr)
	if err != nil {
		return nil, err
	}
	return &retrievalmarket.Ask{
		PricePerByte:            ask.PricePerByte,
		UnsealPrice:             ask.UnsealPrice,
		PaymentInterval:         ask.PaymentInterval,
		PaymentIntervalIncrease: ask.PaymentIntervalIncrease,
	}, nil
}

func (m *MarketNodeImpl) MarketListDataTransfers(ctx context.Context) ([]types.DataTransferChannel, error) {
	inProgressChannels, err := m.DataTransfer.InProgressChannels(ctx)
	if err != nil {
		return nil, err
	}

	apiChannels := make([]types.DataTransferChannel, 0, len(inProgressChannels))
	for _, channelState := range inProgressChannels {
		apiChannels = append(apiChannels, types.NewDataTransferChannel(m.Host.ID(), channelState))
	}

	return apiChannels, nil
}

func (m *MarketNodeImpl) MarketDataTransferUpdates(ctx context.Context) (<-chan types.DataTransferChannel, error) {
	channels := make(chan types.DataTransferChannel)

	unsub := m.DataTransfer.SubscribeToEvents(func(evt datatransfer.Event, channelState datatransfer.ChannelState) {
		channel := types.NewDataTransferChannel(m.Host.ID(), channelState)
		select {
		case <-ctx.Done():
		case channels <- channel:
		}
	})

	go func() {
		defer unsub()
		<-ctx.Done()
	}()

	return channels, nil
}

func (m *MarketNodeImpl) MarketRestartDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error {
	selfPeer := m.Host.ID()
	if isInitiator {
		return m.DataTransfer.RestartDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: selfPeer, Responder: otherPeer, ID: transferID})
	}
	return m.DataTransfer.RestartDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: otherPeer, Responder: selfPeer, ID: transferID})
}

func (m *MarketNodeImpl) MarketCancelDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error {
	selfPeer := m.Host.ID()
	if isInitiator {
		return m.DataTransfer.CloseDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: selfPeer, Responder: otherPeer, ID: transferID})
	}
	return m.DataTransfer.CloseDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: otherPeer, Responder: selfPeer, ID: transferID})
}

func (m *MarketNodeImpl) MarketPendingDeals(ctx context.Context) ([]types.PendingDealInfo, error) {
	dealInfos := m.DealPublisher.PendingDeals()
	ret := make([]types.PendingDealInfo, 0, len(dealInfos))
	for addr, dealInfo := range dealInfos {
		if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err == nil {
			ret = append(ret, dealInfo)
		}
	}
	return ret, nil
}

func (m *MarketNodeImpl) MarketPublishPendingDeals(_ context.Context) error {
	m.DealPublisher.ForcePublishPendingDeals()
	return nil
}

func (m *MarketNodeImpl) PiecesListPieces(ctx context.Context) ([]cid.Cid, error) {
	return m.Repo.StorageDealRepo().ListPieceInfoKeys(ctx)
}

func (m *MarketNodeImpl) PiecesListCidInfos(ctx context.Context) ([]cid.Cid, error) {
	return m.Repo.CidInfoRepo().ListCidInfoKeys(ctx)
}

func (m *MarketNodeImpl) PiecesGetPieceInfo(ctx context.Context, pieceCid cid.Cid) (*piecestore.PieceInfo, error) {
	pi, err := m.Repo.StorageDealRepo().GetPieceInfo(ctx, pieceCid)
	if err != nil {
		return nil, err
	}
	return pi, nil
}

func (m *MarketNodeImpl) PiecesGetCIDInfo(ctx context.Context, payloadCid cid.Cid) (*piecestore.CIDInfo, error) {
	ci, err := m.Repo.CidInfoRepo().GetCIDInfo(ctx, payloadCid)
	if err != nil {
		return nil, err
	}

	return &ci, nil
}

func (m *MarketNodeImpl) DealsConsiderOnlineStorageDeals(ctx context.Context, mAddr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return false, err
	}
	return m.ConsiderOnlineStorageDealsConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetConsiderOnlineStorageDeals(ctx context.Context, mAddr address.Address, b bool) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetConsiderOnlineStorageDealsConfigFunc(mAddr, b)
}

func (m *MarketNodeImpl) DealsConsiderOnlineRetrievalDeals(ctx context.Context, mAddr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return false, err
	}
	return m.ConsiderOnlineRetrievalDealsConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetConsiderOnlineRetrievalDeals(ctx context.Context, mAddr address.Address, b bool) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetConsiderOnlineRetrievalDealsConfigFunc(mAddr, b)
}

func (m *MarketNodeImpl) DealsPieceCidBlocklist(ctx context.Context, mAddr address.Address) ([]cid.Cid, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return nil, err
	}
	return m.StorageDealPieceCidBlocklistConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetPieceCidBlocklist(ctx context.Context, mAddr address.Address, cids []cid.Cid) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetStorageDealPieceCidBlocklistConfigFunc(mAddr, cids)
}

func (m *MarketNodeImpl) DealsConsiderOfflineStorageDeals(ctx context.Context, mAddr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return false, err
	}
	return m.ConsiderOfflineStorageDealsConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetConsiderOfflineStorageDeals(ctx context.Context, mAddr address.Address, b bool) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetConsiderOfflineStorageDealsConfigFunc(mAddr, b)
}

func (m *MarketNodeImpl) DealsConsiderOfflineRetrievalDeals(ctx context.Context, mAddr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return false, err
	}
	return m.ConsiderOfflineRetrievalDealsConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetConsiderOfflineRetrievalDeals(ctx context.Context, mAddr address.Address, b bool) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetConsiderOfflineRetrievalDealsConfigFunc(mAddr, b)
}

func (m *MarketNodeImpl) DealsConsiderVerifiedStorageDeals(ctx context.Context, mAddr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return false, err
	}
	return m.ConsiderVerifiedStorageDealsConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetConsiderVerifiedStorageDeals(ctx context.Context, mAddr address.Address, b bool) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetConsiderVerifiedStorageDealsConfigFunc(mAddr, b)
}

func (m *MarketNodeImpl) DealsConsiderUnverifiedStorageDeals(ctx context.Context, mAddr address.Address) (bool, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return false, err
	}
	return m.ConsiderUnverifiedStorageDealsConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetConsiderUnverifiedStorageDeals(ctx context.Context, mAddr address.Address, b bool) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetConsiderUnverifiedStorageDealsConfigFunc(mAddr, b)
}

func (m *MarketNodeImpl) SectorGetExpectedSealDuration(ctx context.Context, mAddr address.Address) (time.Duration, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return 0, err
	}
	return m.GetExpectedSealDurationFunc(mAddr)
}

func (m *MarketNodeImpl) SectorSetExpectedSealDuration(ctx context.Context, mAddr address.Address, duration time.Duration) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetExpectedSealDurationFunc(mAddr, duration)
}

func (m *MarketNodeImpl) DealsMaxStartDelay(ctx context.Context, mAddr address.Address) (time.Duration, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return 0, err
	}
	return m.GetMaxDealStartDelayFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetMaxStartDelay(ctx context.Context, mAddr address.Address, duration time.Duration) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetMaxDealStartDelayFunc(mAddr, duration)
}

func (m *MarketNodeImpl) DealsPublishMsgPeriod(ctx context.Context, mAddr address.Address) (time.Duration, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return 0, err
	}
	return m.PublishMsgPeriodConfigFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetPublishMsgPeriod(ctx context.Context, mAddr address.Address, duration time.Duration) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetPublishMsgPeriodConfigFunc(mAddr, duration)
}

func (m *MarketNodeImpl) MarketDataTransferPath(ctx context.Context, mAddr address.Address) (string, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return "", err
	}
	return m.TransferPathFunc(mAddr)
}

func (m *MarketNodeImpl) MarketSetDataTransferPath(ctx context.Context, mAddr address.Address, path string) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetTransferPathFunc(mAddr, path)
}

func (m *MarketNodeImpl) MarketMaxDealsPerPublishMsg(ctx context.Context, mAddr address.Address) (uint64, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return 0, err
	}
	return m.MaxDealsPerPublishMsgFunc(mAddr)
}

func (m *MarketNodeImpl) MarketSetMaxDealsPerPublishMsg(ctx context.Context, mAddr address.Address, num uint64) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetMaxDealsPerPublishMsgFunc(mAddr, num)
}

func (m *MarketNodeImpl) DealsMaxProviderCollateralMultiplier(ctx context.Context, mAddr address.Address) (uint64, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return 0, err
	}
	return m.MaxProviderCollateralMultiplierFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetMaxProviderCollateralMultiplier(ctx context.Context, mAddr address.Address, c uint64) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetMaxProviderCollateralMultiplierFunc(mAddr, c)
}

func (m *MarketNodeImpl) DealsMaxPublishFee(ctx context.Context, mAddr address.Address) (vTypes.FIL, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return vTypes.FIL(vTypes.ZeroFIL), err
	}
	return m.MaxPublishDealsFeeFunc(mAddr)
}

func (m *MarketNodeImpl) DealsSetMaxPublishFee(ctx context.Context, mAddr address.Address, fee vTypes.FIL) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetMaxPublishDealsFeeFunc(mAddr, fee)
}

func (m *MarketNodeImpl) MarketMaxBalanceAddFee(ctx context.Context, mAddr address.Address) (vTypes.FIL, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return vTypes.FIL(vTypes.ZeroFIL), err
	}
	return m.MaxMarketBalanceAddFeeFunc(mAddr)
}

func (m *MarketNodeImpl) MarketSetMaxBalanceAddFee(ctx context.Context, mAddr address.Address, fee vTypes.FIL) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return err
	}
	return m.SetMaxMarketBalanceAddFeeFunc(mAddr, fee)
}

func (m *MarketNodeImpl) MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*vTypes.MsgLookup, error) {
	// WaitMsg method has been replace in messager mode
	return m.Messager.WaitMsg(ctx, mid, constants.MessageConfidence, constants.LookbackNoLimit, false)
}

func (m *MarketNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *vTypes.MessageSendSpec) (cid.Cid, error) {
	if err := jwtclient.CheckPermissionBySigner(ctx, m.AuthClient, msg.From); err != nil {
		return cid.Undef, err
	}
	var spec *vTypes.MessageSendSpec
	if meta != nil {
		spec = &vTypes.MessageSendSpec{
			MaxFee:            meta.MaxFee,
			GasOverEstimation: meta.GasOverEstimation,
		}
	}
	return m.Messager.PushMessage(ctx, msg, spec)
}

func (m *MarketNodeImpl) MessagerGetMessage(ctx context.Context, mid cid.Cid) (*vTypes.Message, error) {
	return m.Messager.GetMessage(ctx, mid)
}

func (m *MarketNodeImpl) listDeals(ctx context.Context, addrs []address.Address) ([]*vTypes.MarketDeal, error) {
	ts, err := m.FullNode.ChainHead(ctx)
	if err != nil {
		return nil, err
	}

	allDeals, err := m.FullNode.StateMarketDeals(ctx, ts.Key())
	if err != nil {
		return nil, err
	}

	var out []*vTypes.MarketDeal

	has := func(addr address.Address) bool {
		for _, a := range addrs {
			if a == addr {
				return true
			}
		}

		return false
	}

	for _, deal := range allDeals {
		if m.UserMgr.Has(ctx, deal.Proposal.Provider) && has(deal.Proposal.Provider) {
			out = append(out, deal)
		}
	}

	return out, nil
}

func (m *MarketNodeImpl) NetAddrsListen(context.Context) (peer.AddrInfo, error) {
	return peer.AddrInfo{
		ID:    m.Host.ID(),
		Addrs: m.Host.Addrs(),
	}, nil
}

func (m *MarketNodeImpl) ID(context.Context) (peer.ID, error) {
	return m.Host.ID(), nil
}

func (m *MarketNodeImpl) DagstoreListShards(_ context.Context) ([]types.DagstoreShardInfo, error) {
	info := m.DAGStore.AllShardsInfo()
	ret := make([]types.DagstoreShardInfo, 0, len(info))
	for k, i := range info {
		ret = append(ret, types.DagstoreShardInfo{
			Key:   k.String(),
			State: i.ShardState.String(),
			Error: func() string {
				if i.Error == nil {
					return ""
				}
				return i.Error.Error()
			}(),
		})
	}

	// order by key.
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].Key < ret[j].Key
	})

	return ret, nil
}

func (m *MarketNodeImpl) DagstoreInitializeShard(ctx context.Context, key string) error {
	// check whether key valid
	cidKey, err := cid.Decode(key)
	if err != nil {
		return err
	}
	_, err = m.Repo.StorageDealRepo().GetPieceInfo(ctx, cidKey)
	if err != nil {
		return err
	}

	// check whether shard info exit
	k := shard.KeyFromString(key)
	info, err := m.DAGStore.GetShardInfo(k)
	if err != nil && err != dagstore.ErrShardUnknown {
		return fmt.Errorf("failed to get shard info: %w", err)
	}

	if st := info.ShardState; st != dagstore.ShardStateNew {
		return fmt.Errorf("cannot initialize shard; expected state ShardStateNew, was: %s", st.String())
	}

	bs, err := m.DAGStoreWrapper.LoadShard(ctx, cidKey)
	if err != nil {
		return err
	}
	return bs.Close()
}

func (m *MarketNodeImpl) DagstoreInitializeAll(ctx context.Context, params types.DagstoreInitializeAllParams) (<-chan types.DagstoreInitializeAllEvent, error) {
	deals, err := m.Repo.StorageDealRepo().GetDealByAddrAndStatus(ctx, address.Undef, storageprovider.ReadyRetrievalDealStatus...)
	if err != nil {
		return nil, err
	}
	// are we initializing only unsealed pieces?
	onlyUnsealed := !params.IncludeSealed

	var toInitialize []string
	for _, deal := range deals {
		pieceCid := deal.ClientDealProposal.Proposal.PieceCID
		info, err := m.DAGStore.GetShardInfo(shard.KeyFromCID(pieceCid))
		if err != nil && err != dagstore.ErrShardUnknown {
			return nil, err
		}

		if info.ShardState != dagstore.ShardStateNew {
			continue
		}

		// if we're initializing only unsealed pieces, check if there's an
		// unsealed deal for this piece available.
		if onlyUnsealed {
			_, err = m.PieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
			if err != nil {
				// todo unseal
				log.Warnw("DagstoreInitializeAll: failed to get unsealed status; skipping deal", "piece cid", pieceCid, "error", err)
				continue
			}
		}
		// todo trigger unseal
		// yes, we're initializing this shard.
		toInitialize = append(toInitialize, pieceCid.String())
	}

	return m.dagstoreLoadShards(ctx, toInitialize, params.MaxConcurrency)
}

func (m *MarketNodeImpl) DagstoreInitializeStorage(ctx context.Context, storageName string, params types.DagstoreInitializeAllParams) (<-chan types.DagstoreInitializeAllEvent, error) {
	storage, err := m.PieceStorageMgr.GetPieceStorageByName(storageName)
	if err != nil {
		return nil, err
	}
	resourceIds, err := storage.ListResourceIds(ctx)
	if err != nil {
		return nil, err
	}

	var toInitialize []string
	for _, resource := range resourceIds {
		pieceCid, err := cid.Decode(resource)
		if err != nil {
			log.Warnf("resource name (%s) was not a valid piece cid %v", resource, err)
			continue
		}
		pieceInfo, err := m.Repo.StorageDealRepo().GetPieceInfo(ctx, pieceCid)
		if err != nil || (pieceInfo != nil && len(pieceInfo.Deals) == 0) {
			log.Warnf("piece cid %s not in storage deals", pieceCid)
			continue
		}

		_, err = m.DAGStore.GetShardInfo(shard.KeyFromString(resource))
		if err != nil && !errors.Is(err, dagstore.ErrShardUnknown) {
			return nil, err
		}

		toInitialize = append(toInitialize, resource)
	}

	return m.dagstoreLoadShards(ctx, toInitialize, params.MaxConcurrency)
}

func (m *MarketNodeImpl) DagstoreDestroyShard(ctx context.Context, key string) error {
	opts := dagstore.DestroyOpts{}
	sr := make(chan dagstore.ShardResult, 1)

	shardKey := shard.KeyFromString(key)
	if _, err := m.DAGStore.GetShardInfo(shardKey); err != nil {
		return fmt.Errorf("query shard failed: %v", err)
	}

	if err := m.DAGStore.DestroyShard(ctx, shardKey, sr, opts); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-sr:
		return r.Error
	}
}

func (m *MarketNodeImpl) dagstoreLoadShards(ctx context.Context, toInitialize []string, concurrency int) (<-chan types.DagstoreInitializeAllEvent, error) {
	// prepare the thottler tokens.
	var throttle chan struct{}
	if c := concurrency; c > 0 {
		throttle = make(chan struct{}, c)
		for i := 0; i < c; i++ {
			throttle <- struct{}{}
		}
	}

	total := len(toInitialize)
	if total == 0 {
		out := make(chan types.DagstoreInitializeAllEvent)
		close(out)
		return out, nil
	}

	// response channel must be closed when we're done, or the context is cancelled.
	// this buffering is necessary to prevent inflight children goroutines from
	// publishing to a closed channel (res) when the context is cancelled.
	out := make(chan types.DagstoreInitializeAllEvent, 32) // internal buffer.
	res := make(chan types.DagstoreInitializeAllEvent, 32) // returned to caller.

	// pump events back to caller.
	// two events per shard.
	go func() {
		defer close(res)

		for i := 0; i < total*2; i++ {
			select {
			case res <- <-out:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for i, k := range toInitialize {
			if throttle != nil {
				select {
				case <-throttle:
					// acquired a throttle token, proceed.
				case <-ctx.Done():
					return
				}
			}

			go func(k string, i int) {
				r := types.DagstoreInitializeAllEvent{
					Key:     k,
					Event:   "start",
					Total:   total,
					Current: i + 1, // start with 1
				}
				select {
				case out <- r:
				case <-ctx.Done():
					return
				}

				err := m.DagstoreInitializeShard(ctx, k)

				if throttle != nil {
					throttle <- struct{}{}
				}

				r.Event = "end"
				if err == nil {
					r.Success = true
				} else {
					r.Success = false
					r.Error = err.Error()
				}

				select {
				case out <- r:
				case <-ctx.Done():
				}
			}(k, i)
		}
	}()

	return res, nil
}

func (m *MarketNodeImpl) DagstoreRecoverShard(ctx context.Context, key string) error {
	k := shard.KeyFromString(key)

	info, err := m.DAGStore.GetShardInfo(k)
	if err != nil {
		return fmt.Errorf("failed to get shard info: %w", err)
	}
	if st := info.ShardState; st != dagstore.ShardStateErrored {
		return fmt.Errorf("cannot recover shard; expected state ShardStateErrored, was: %s", st.String())
	}

	ch := make(chan dagstore.ShardResult, 1)
	if err = m.DAGStore.RecoverShard(ctx, k, ch, dagstore.RecoverOpts{}); err != nil {
		return fmt.Errorf("failed to recover shard: %w", err)
	}

	var res dagstore.ShardResult
	select {
	case res = <-ch:
	case <-ctx.Done():
		return ctx.Err()
	}

	return res.Error
}

func (m *MarketNodeImpl) DagstoreGC(ctx context.Context) ([]types.DagstoreShardResult, error) {
	if m.DAGStore == nil {
		return nil, fmt.Errorf("dagstore not available on this node")
	}

	res, err := m.DAGStore.GC(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to gc: %w", err)
	}

	ret := make([]types.DagstoreShardResult, 0, len(res.Shards))
	for k, err := range res.Shards {
		r := types.DagstoreShardResult{Key: k.String()}
		if err == nil {
			r.Success = true
		} else {
			r.Success = false
			r.Error = err.Error()
		}
		ret = append(ret, r)
	}

	return ret, nil
}

func (m *MarketNodeImpl) GetUnPackedDeals(ctx context.Context, miner address.Address, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return nil, err
	}
	return m.DealAssigner.GetUnPackedDeals(ctx, miner, spec)
}

func (m *MarketNodeImpl) AssignUnPackedDeals(ctx context.Context, sid abi.SectorID, ssize abi.SectorSize, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error) {
	mAddr, err := address.NewIDAddress(uint64(sid.Miner))
	if err != nil {
		return nil, err
	}
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return nil, err
	}

	head, err := m.FullNode.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain head %w", err)
	}
	return m.DealAssigner.AssignUnPackedDeals(ctx, sid, ssize, head.Height(), spec)
}

// ReleaseDeals is used to release the deals that have been assigned by AssignUnPackedDeals method.
func (m *MarketNodeImpl) ReleaseDeals(ctx context.Context, miner address.Address, deals []abi.DealID) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return err
	}
	return m.DealAssigner.ReleaseDeals(ctx, miner, deals)
}

func (m *MarketNodeImpl) MarkDealsAsPacking(ctx context.Context, miner address.Address, deals []abi.DealID) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return err
	}
	return m.DealAssigner.MarkDealsAsPacking(ctx, miner, deals)
}

func (m *MarketNodeImpl) UpdateDealOnPacking(ctx context.Context, miner address.Address, dealId abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return err
	}
	return m.DealAssigner.UpdateDealOnPacking(ctx, miner, dealId, sectorid, offset)
}

func (m *MarketNodeImpl) AssignDeals(ctx context.Context, sid abi.SectorID, ssize abi.SectorSize, spec *types.GetDealSpec) ([]*types.DealInfoV2, error) {
	mAddr, err := address.NewIDAddress(uint64(sid.Miner))
	if err != nil {
		return nil, err
	}
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, mAddr); err != nil {
		return nil, err
	}

	head, err := m.FullNode.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain head %w", err)
	}
	return m.DealAssigner.AssignDeals(ctx, sid, ssize, head.Height(), spec)
}

func (m *MarketNodeImpl) ReleaseDirectDeals(ctx context.Context, miner address.Address, allocationIDs []vTypes.AllocationId) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return err
	}
	return m.DealAssigner.ReleaseDirectDeals(ctx, miner, allocationIDs)
}

func (m *MarketNodeImpl) UpdateDealStatus(ctx context.Context, miner address.Address, dealId abi.DealID, pieceStatus types.PieceStatus, dealStatus storagemarket.StorageDealStatus) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return err
	}
	return m.DealAssigner.UpdateDealStatus(ctx, miner, dealId, pieceStatus, dealStatus)
}

func (m *MarketNodeImpl) UpdateStorageDealPayloadSize(ctx context.Context, dealProposal cid.Cid, payloadSize uint64) error {
	deal, err := m.Repo.StorageDealRepo().GetDeal(ctx, dealProposal)
	if err != nil {
		return err
	}
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, deal.Proposal.Provider); err != nil {
		return err
	}

	deal.PayloadSize = payloadSize
	return m.Repo.StorageDealRepo().SaveDeal(ctx, deal)
}

func (m *MarketNodeImpl) DealsImportData(ctx context.Context, ref types.ImportDataRef, skipCommP bool) error {
	deal, _, err := storageprovider.GetDealByDataRef(ctx, m.Repo.StorageDealRepo(), &ref)
	if err != nil {
		return err
	}
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, deal.Proposal.Provider); err != nil {
		return err
	}

	res, err := m.DealsBatchImportData(ctx, types.ImportDataRefs{
		Refs:      []*types.ImportDataRef{&ref},
		SkipCommP: skipCommP,
	})
	if err != nil {
		return err
	}
	if len(res[0].Message) > 0 {
		return fmt.Errorf(res[0].Message)
	}

	return nil
}

func (m *MarketNodeImpl) GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*types.DealInfo, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return nil, err
	}
	return m.DealAssigner.GetDeals(ctx, miner, pageIndex, pageSize)
}

func (m *MarketNodeImpl) PaychVoucherList(ctx context.Context, pch address.Address) ([]*vTypes.SignedVoucher, error) {
	ci, err := m.Repo.PaychChannelInfoRepo().GetChannelByAddress(ctx, pch)
	if err != nil {
		return nil, err
	}
	if err := jwtclient.CheckPermissionBySigner(ctx, m.AuthClient, ci.Control); err != nil {
		return nil, err
	}
	return m.PaychAPI.PaychVoucherList(ctx, pch)
}

func (m *MarketNodeImpl) AddFsPieceStorage(_ context.Context, name string, path string, readonly bool) error {
	ifs := &config.FsPieceStorage{ReadOnly: readonly, Path: path, Name: name}
	fsps, err := piecestorage.NewFsPieceStorage(ifs)
	if err != nil {
		return err
	}
	// add in memory
	err = m.PieceStorageMgr.AddPieceStorage(fsps)
	if err != nil {
		return err
	}

	// add to config
	return m.Config.AddFsPieceStorage(ifs)
}

func (m *MarketNodeImpl) AddS3PieceStorage(_ context.Context, name, endpoit, bucket, subdir, accessKeyID, secretAccessKey, token string, readonly bool) error {
	ifs := &config.S3PieceStorage{
		ReadOnly:  readonly,
		EndPoint:  endpoit,
		Name:      name,
		Bucket:    bucket,
		SubDir:    subdir,
		AccessKey: accessKeyID,
		SecretKey: secretAccessKey,
		Token:     token,
	}
	s3ps, err := piecestorage.NewS3PieceStorage(ifs)
	if err != nil {
		return err
	}
	// add in memory
	err = m.PieceStorageMgr.AddPieceStorage(s3ps)
	if err != nil {
		return err
	}

	// add to config
	return m.Config.AddS3PieceStorage(ifs)
}

func (m *MarketNodeImpl) ListPieceStorageInfos(_ context.Context) types.PieceStorageInfos {
	return m.PieceStorageMgr.ListStorageInfos()
}

func (m *MarketNodeImpl) RemovePieceStorage(_ context.Context, name string) error {
	err := m.PieceStorageMgr.RemovePieceStorage(name)
	if err != nil {
		return err
	}

	return m.Config.RemovePieceStorage(name)
}

func (m *MarketNodeImpl) DealsImport(ctx context.Context, deals []*types.MinerDeal) error {
	if len(deals) == 0 {
		return nil
	}

	addrDeals := make(map[address.Address][]*types.MinerDeal)
	for _, deal := range deals {
		addrDeals[deal.Proposal.Provider] = append(addrDeals[deal.Proposal.Provider], deal)
	}

	var errs *multierror.Error
	valid := make(map[address.Address][]*types.MinerDeal, len(addrDeals))
	for addr, d := range addrDeals {
		if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		valid[addr] = d
	}
	errs = multierror.Append(errs, m.StorageProvider.ImportDeals(ctx, valid))

	return errs.ErrorOrNil()
}

func (m *MarketNodeImpl) Version(_ context.Context) (vTypes.Version, error) {
	return vTypes.Version{Version: version.UserVersion()}, nil
}

func (m *MarketNodeImpl) GetStorageDealStatistic(ctx context.Context, miner address.Address) (*types.StorageDealStatistic, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return nil, err
	}
	statistic, err := m.Repo.StorageDealRepo().GroupStorageDealNumberByStatus(ctx, miner)
	if err != nil {
		return nil, err
	}
	return &types.StorageDealStatistic{DealsStatus: statistic}, nil
}

func (m *MarketNodeImpl) GetRetrievalDealStatistic(ctx context.Context, miner address.Address) (*types.RetrievalDealStatistic, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, miner); err != nil {
		return nil, err
	}
	statistic, err := m.Repo.RetrievalDealRepo().GroupRetrievalDealNumberByStatus(ctx, miner)
	if err != nil {
		return nil, err
	}
	return &types.RetrievalDealStatistic{DealsStatus: statistic}, nil
}

func (m *MarketNodeImpl) MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error) {
	if err := jwtclient.CheckPermissionBySigner(ctx, m.AuthClient, wallet); err != nil {
		return cid.Undef, err
	}
	return m.FundAPI.MarketAddBalance(ctx, wallet, addr, amt)
}

func (m *MarketNodeImpl) MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error) {
	// we don't check permission on wallet because wallet will be the addr that the fund withdraw in
	//but signing by wallet will be called automatically without permission check
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
		return cid.Undef, err
	}
	return m.FundAPI.MarketWithdraw(ctx, wallet, addr, amt)
}

func (m *MarketNodeImpl) MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt vTypes.BigInt) (cid.Cid, error) {
	if err := jwtclient.CheckPermissionBySigner(ctx, m.AuthClient, wallet); err != nil {
		return cid.Undef, err
	}
	return m.FundAPI.MarketReserveFunds(ctx, wallet, addr, amt)
}

func (m *MarketNodeImpl) MarketReleaseFunds(ctx context.Context, addr address.Address, amt vTypes.BigInt) error {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
		return err
	}
	return m.FundAPI.MarketReleaseFunds(ctx, addr, amt)
}

func (m *MarketNodeImpl) MarketGetReserved(ctx context.Context, addr address.Address) (vTypes.BigInt, error) {
	if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, addr); err != nil {
		return vTypes.BigInt{}, err
	}
	return m.FundAPI.MarketGetReserved(ctx, addr)
}

func (m *MarketNodeImpl) DealsBatchImportData(ctx context.Context, refs types.ImportDataRefs) ([]*types.ImportDataResult, error) {
	refLen := len(refs.Refs)
	results := make([]*types.ImportDataResult, 0, refLen)
	validRefs := make([]*types.ImportDataRef, 0, refLen)

	for _, ref := range refs.Refs {
		deal, target, err := storageprovider.GetDealByDataRef(ctx, m.Repo.StorageDealRepo(), ref)
		if err != nil {
			results = append(results, &types.ImportDataResult{
				Target:  target,
				Message: err.Error(),
			})
			continue
		}
		// todo: cache provider permission, avoid repeated permission check
		if err := jwtclient.CheckPermissionByMiner(ctx, m.AuthClient, deal.Proposal.Provider); err != nil {
			results = append(results, &types.ImportDataResult{
				Target:  target,
				Message: err.Error(),
			})
			continue
		}
		validRefs = append(validRefs, ref)
	}

	res, err := m.StorageProvider.ImportDataForDeals(ctx, validRefs, refs.SkipCommP)
	if err != nil {
		return nil, err
	}
	results = append(results, res...)

	return results, nil
}

func (m *MarketNodeImpl) ImportDirectDeal(ctx context.Context, dealParams *types.DirectDealParams) error {
	if len(dealParams.DealParams) == 0 {
		return errors.New("deal params is empty")
	}
	return m.DirectDealProvider.ImportDeals(ctx, dealParams)
}

func (m *MarketNodeImpl) GetDirectDeal(ctx context.Context, id uuid.UUID) (*types.DirectDeal, error) {
	return m.Repo.DirectDealRepo().GetDeal(ctx, id)
}

func (m *MarketNodeImpl) GetDirectDealByAllocationID(ctx context.Context, id vTypes.AllocationId) (*types.DirectDeal, error) {
	return m.Repo.DirectDealRepo().GetDealByAllocationID(ctx, uint64(id))
}

func (m *MarketNodeImpl) ListDirectDeals(ctx context.Context, queryParams types.DirectDealQueryParams) ([]*types.DirectDeal, error) {
	return m.Repo.DirectDealRepo().ListDeal(ctx, queryParams)
}

func (m *MarketNodeImpl) UpdateDirectDealState(ctx context.Context, id uuid.UUID, state types.DirectDealState) error {
	deal, err := m.Repo.DirectDealRepo().GetDeal(ctx, id)
	if err != nil {
		return err
	}
	deal.State = state

	return m.Repo.DirectDealRepo().SaveDeal(ctx, deal)
}

func (m *MarketNodeImpl) IndexerAnnounceAllDeals(ctx context.Context, minerAddr address.Address) error {
	return m.IndexProviderMgr.IndexAnnounceAllDeals(ctx, minerAddr)
}

func (m *MarketNodeImpl) getDeal(ctx context.Context, contextID []byte) (any, bool, error) {
	propCID, err := cid.Cast(contextID)
	if err == nil {
		deal, err := m.Repo.StorageDealRepo().GetDeal(ctx, propCID)
		if err != nil {
			return address.Address{}, false, err
		}
		return deal, false, nil
	}
	dealUUID, err := uuid.FromBytes(contextID)
	if err != nil {
		return address.Address{}, false, err
	}

	directDeal, err := m.Repo.DirectDealRepo().GetDeal(ctx, dealUUID)
	if err == nil {
		return directDeal, true, nil
	}

	deal, err := m.Repo.StorageDealRepo().GetDealByUUID(ctx, dealUUID)
	if err != nil {
		return address.Address{}, false, err
	}

	return deal, false, nil
}

func (m *MarketNodeImpl) IndexerListMultihashes(ctx context.Context, contextID []byte) ([]multihash.Multihash, error) {
	deal, isDDO, err := m.getDeal(ctx, contextID)
	if err != nil {
		return nil, err
	}
	var miner address.Address
	if isDDO {
		miner = deal.(*types.DirectDeal).Provider
	} else {
		miner = deal.(*types.MinerDeal).Proposal.Provider
	}

	it, err := m.IndexProviderMgr.MultihashLister(ctx, miner, "", contextID)
	if err != nil {
		return nil, err
	}

	var mhs []multihash.Multihash
	mh, err := it.Next()
	for {
		if err != nil {
			if errors.Is(err, io.EOF) {
				return mhs, nil
			}
			return nil, err
		}
		mhs = append(mhs, mh)

		mh, err = it.Next()
	}
}

func (m *MarketNodeImpl) IndexerAnnounceLatest(ctx context.Context) (cid.Cid, error) {
	var c cid.Cid
	var err error
	for _, miner := range m.Config.Miners {
		c, err = m.IndexProviderMgr.IndexerAnnounceLatest(ctx, address.Address(miner.Addr))
		if err != nil {
			return c, err
		}
	}

	return c, nil
}

func (m *MarketNodeImpl) IndexerAnnounceLatestHttp(ctx context.Context, urls []string) (cid.Cid, error) {
	var c cid.Cid
	var err error
	for _, miner := range m.Config.Miners {
		c, err = m.IndexProviderMgr.IndexerAnnounceLatestHttp(ctx, address.Address(miner.Addr), urls)
		if err != nil {
			return c, err
		}
	}

	return c, nil
}

func (m *MarketNodeImpl) IndexerAnnounceDealRemoved(ctx context.Context, contextID []byte) (cid.Cid, error) {
	deal, isDDO, err := m.getDeal(ctx, contextID)
	if err != nil {
		return cid.Undef, err
	}
	var miner address.Address
	if isDDO {
		miner = deal.(*types.DirectDeal).Provider
	} else {
		miner = deal.(*types.MinerDeal).Proposal.Provider
	}

	return m.IndexProviderMgr.AnnounceDealRemoved(ctx, miner, contextID)
}

func (m *MarketNodeImpl) IndexerAnnounceDeal(ctx context.Context, contextID []byte) (cid.Cid, error) {
	deal, isDDO, err := m.getDeal(ctx, contextID)
	if err != nil {
		return cid.Undef, err
	}
	if isDDO {
		return m.IndexProviderMgr.AnnounceDirectDeal(ctx, deal.(*types.DirectDeal))
	}

	return m.IndexProviderMgr.AnnounceDeal(ctx, deal.(*types.MinerDeal))
}
