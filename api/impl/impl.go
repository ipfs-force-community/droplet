package impl

import (
	"context"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/constants"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/piece"
	storageadapter2 "github.com/filecoin-project/venus-market/storageadapter"
	"github.com/filecoin-project/venus-market/types"
	mTypes "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/client/apiface"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"os"
	"time"
)

var _ api.MarketFullNode = (*MarketNodeImpl)(nil)

type MarketNodeImpl struct {
	FundAPI
	fx.In
	Cfg               *config.MarketConfig
	FullNode          apiface.FullNode
	Host              host.Host
	StorageProvider   storagemarket.StorageProvider
	RetrievalProvider retrievalmarket.RetrievalProvider
	DataTransfer      network.ProviderDataTransfer
	DealPublisher     *storageadapter2.DealPublisher
	PieceStore        piece.ExtendPieceStore
	Messager          clients.IMessager

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
	/*	SetSealingConfigFunc                        dtypes.SetSealingConfigFunc
		GetSealingConfigFunc                        dtypes.GetSealingConfigFunc  */
	GetExpectedSealDurationFunc config.GetExpectedSealDurationFunc
	SetExpectedSealDurationFunc config.SetExpectedSealDurationFunc
}

func (m MarketNodeImpl) ActorAddress(ctx context.Context) (address.Address, error) {
	return address.NewFromString(m.Cfg.MinerAddress)
}

func (m MarketNodeImpl) ActorSectorSize(ctx context.Context, addr address.Address) (abi.SectorSize, error) {
	mAddr, err := address.NewFromString(m.Cfg.MinerAddress)
	if err != nil {
		return 0, err
	}
	minerInfo, err := m.FullNode.StateMinerInfo(ctx, mAddr, vTypes.EmptyTSK)
	if err != nil {
		return 0, err
	}
	return minerInfo.SectorSize, nil
}

func (m MarketNodeImpl) MarketImportDealData(ctx context.Context, propCid cid.Cid, path string) error {
	fi, err := os.Open(path)
	if err != nil {
		return xerrors.Errorf("failed to open file: %w", err)
	}
	defer fi.Close() //nolint:errcheck

	return m.StorageProvider.ImportDataForDeal(ctx, propCid, fi)
}

func (m MarketNodeImpl) MarketListDeals(ctx context.Context) ([]types.MarketDeal, error) {
	return m.listDeals(ctx)
}

func (m MarketNodeImpl) MarketListRetrievalDeals(ctx context.Context) ([]retrievalmarket.ProviderDealState, error) {
	var out []retrievalmarket.ProviderDealState
	deals := m.RetrievalProvider.ListDeals()

	for _, deal := range deals {
		if deal.ChannelID != nil {
			if deal.ChannelID.Initiator == "" || deal.ChannelID.Responder == "" {
				deal.ChannelID = nil // don't try to push unparsable peer IDs over jsonrpc
			}
		}
		out = append(out, deal)
	}

	return out, nil
}

func (m MarketNodeImpl) MarketGetDealUpdates(ctx context.Context) (<-chan storagemarket.MinerDeal, error) {
	results := make(chan storagemarket.MinerDeal)
	unsub := m.StorageProvider.SubscribeToEvents(func(evt storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
		select {
		case results <- deal:
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

func (m MarketNodeImpl) MarketListIncompleteDeals(ctx context.Context) ([]storagemarket.MinerDeal, error) {
	return m.StorageProvider.ListLocalDeals()
}

func (m MarketNodeImpl) MarketSetAsk(ctx context.Context, price vTypes.BigInt, verifiedPrice vTypes.BigInt, duration abi.ChainEpoch, minPieceSize abi.PaddedPieceSize, maxPieceSize abi.PaddedPieceSize) error {
	options := []storagemarket.StorageAskOption{
		storagemarket.MinPieceSize(minPieceSize),
		storagemarket.MaxPieceSize(maxPieceSize),
	}

	return m.StorageProvider.SetAsk(price, verifiedPrice, duration, options...)
}

func (m MarketNodeImpl) MarketGetAsk(ctx context.Context) (*storagemarket.SignedStorageAsk, error) {
	return m.StorageProvider.GetAsk(), nil
}

func (m MarketNodeImpl) MarketSetRetrievalAsk(ctx context.Context, rask *retrievalmarket.Ask) error {
	m.RetrievalProvider.SetAsk(rask)
	return nil
}

func (m MarketNodeImpl) MarketGetRetrievalAsk(ctx context.Context) (*retrievalmarket.Ask, error) {
	return m.RetrievalProvider.GetAsk(), nil
}

func (m MarketNodeImpl) MarketListDataTransfers(ctx context.Context) ([]types.DataTransferChannel, error) {
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

func (m MarketNodeImpl) MarketDataTransferUpdates(ctx context.Context) (<-chan types.DataTransferChannel, error) {
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

func (m MarketNodeImpl) MarketRestartDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error {
	selfPeer := m.Host.ID()
	if isInitiator {
		return m.DataTransfer.RestartDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: selfPeer, Responder: otherPeer, ID: transferID})
	}
	return m.DataTransfer.RestartDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: otherPeer, Responder: selfPeer, ID: transferID})
}

func (m MarketNodeImpl) MarketCancelDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error {
	selfPeer := m.Host.ID()
	if isInitiator {
		return m.DataTransfer.CloseDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: selfPeer, Responder: otherPeer, ID: transferID})
	}
	return m.DataTransfer.CloseDataTransferChannel(ctx, datatransfer.ChannelID{Initiator: otherPeer, Responder: selfPeer, ID: transferID})
}

func (m MarketNodeImpl) MarketPendingDeals(ctx context.Context) (types.PendingDealInfo, error) {
	return m.DealPublisher.PendingDeals(), nil
}

func (m MarketNodeImpl) MarketPublishPendingDeals(ctx context.Context) error {
	m.DealPublisher.ForcePublishPendingDeals()
	return nil
}

func (m MarketNodeImpl) PiecesListPieces(ctx context.Context) ([]cid.Cid, error) {
	return m.PieceStore.ListPieceInfoKeys()
}

func (m MarketNodeImpl) PiecesListCidInfos(ctx context.Context) ([]cid.Cid, error) {
	return m.PieceStore.ListCidInfoKeys()
}

func (m MarketNodeImpl) PiecesGetPieceInfo(ctx context.Context, pieceCid cid.Cid) (*piecestore.PieceInfo, error) {
	pi, err := m.PieceStore.GetPieceInfo(pieceCid)
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

func (m MarketNodeImpl) PiecesGetCIDInfo(ctx context.Context, payloadCid cid.Cid) (*piecestore.CIDInfo, error) {
	ci, err := m.PieceStore.GetCIDInfo(payloadCid)
	if err != nil {
		return nil, err
	}

	return &ci, nil
}
func (m MarketNodeImpl) DealsList(ctx context.Context) ([]types.MarketDeal, error) {
	return m.listDeals(ctx)
}

func (m MarketNodeImpl) DealsConsiderOnlineStorageDeals(ctx context.Context) (bool, error) {
	return m.ConsiderOnlineStorageDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderOnlineStorageDeals(ctx context.Context, b bool) error {
	return m.DealsSetConsiderOnlineStorageDeals(ctx, b)
}

func (m MarketNodeImpl) DealsConsiderOnlineRetrievalDeals(ctx context.Context) (bool, error) {
	return m.DealsConsiderOnlineRetrievalDeals(ctx)
}

func (m MarketNodeImpl) DealsSetConsiderOnlineRetrievalDeals(ctx context.Context, b bool) error {
	return m.DealsSetConsiderOnlineRetrievalDeals(ctx, b)
}

func (m MarketNodeImpl) DealsPieceCidBlocklist(ctx context.Context) ([]cid.Cid, error) {
	return m.DealsPieceCidBlocklist(ctx)
}

func (m MarketNodeImpl) DealsSetPieceCidBlocklist(ctx context.Context, cids []cid.Cid) error {
	return m.DealsSetPieceCidBlocklist(ctx, cids)
}

func (m MarketNodeImpl) DealsConsiderOfflineStorageDeals(ctx context.Context) (bool, error) {
	return m.DealsConsiderOfflineStorageDeals(ctx)
}

func (m MarketNodeImpl) DealsSetConsiderOfflineStorageDeals(ctx context.Context, b bool) error {
	return m.DealsSetConsiderOfflineStorageDeals(ctx, b)
}

func (m MarketNodeImpl) DealsConsiderOfflineRetrievalDeals(ctx context.Context) (bool, error) {
	return m.DealsConsiderOfflineRetrievalDeals(ctx)
}

func (m MarketNodeImpl) DealsSetConsiderOfflineRetrievalDeals(ctx context.Context, b bool) error {
	return m.DealsSetConsiderOfflineRetrievalDeals(ctx, b)
}

func (m MarketNodeImpl) DealsConsiderVerifiedStorageDeals(ctx context.Context) (bool, error) {
	return m.DealsConsiderVerifiedStorageDeals(ctx)
}

func (m MarketNodeImpl) DealsSetConsiderVerifiedStorageDeals(ctx context.Context, b bool) error {
	return m.DealsSetConsiderVerifiedStorageDeals(ctx, b)
}

func (m MarketNodeImpl) DealsConsiderUnverifiedStorageDeals(ctx context.Context) (bool, error) {
	return m.DealsConsiderUnverifiedStorageDeals(ctx)
}

func (m MarketNodeImpl) DealsSetConsiderUnverifiedStorageDeals(ctx context.Context, b bool) error {
	return m.DealsSetConsiderUnverifiedStorageDeals(ctx, b)
}

func (m MarketNodeImpl) SectorGetSealDelay(ctx context.Context) (time.Duration, error) {
	return m.SectorGetSealDelay(ctx)
}

func (m MarketNodeImpl) SectorSetExpectedSealDuration(ctx context.Context, duration time.Duration) error {
	return m.SectorSetExpectedSealDuration(ctx, duration)
}

func (m MarketNodeImpl) MessagerWaitMessage(ctx context.Context, uid uuid.UUID) (*mTypes.Message, error) {
	return m.Messager.WaitMessage(ctx, uid.String(), constants.MessageConfidence)
}

func (m MarketNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (uuid.UUID, error) {
	uid := uuid.New()
	_, err := m.Messager.PushMessageWithId(ctx, uid.String(), msg, meta)
	if err != nil {
		return uuid.UUID{}, nil
	}
	return uid, nil
}

func (m MarketNodeImpl) MessagerGetMessage(ctx context.Context, uid uuid.UUID) (*mTypes.Message, error) {
	return m.Messager.GetMessageByUid(ctx, uid.String())
}

func (m MarketNodeImpl) listDeals(ctx context.Context) ([]types.MarketDeal, error) {
	ts, err := m.FullNode.ChainHead(ctx)
	if err != nil {
		return nil, err
	}
	tsk := ts.Key()
	allDeals, err := m.FullNode.StateMarketDeals(ctx, tsk)
	if err != nil {
		return nil, err
	}

	var out []types.MarketDeal

	addr, err := m.ActorAddress(ctx)
	if err != nil {
		return nil, err
	}

	for _, deal := range allDeals {
		if deal.Proposal.Provider == addr {
			out = append(out, deal)
		}
	}

	return out, nil
}

func (m MarketNodeImpl) NetAddrsListen(context.Context) (peer.AddrInfo, error) {
	return peer.AddrInfo{
		ID:    m.Host.ID(),
		Addrs: m.Host.Addrs(),
	}, nil
}

func (m MarketNodeImpl) ID(context.Context) (peer.ID, error) {
	return m.Host.ID(), nil
}

func (m MarketNodeImpl) GetUnPackedDeals(miner address.Address, spec *piece.GetDealSpec) ([]*piece.DealInfo, error) {
	return m.PieceStore.GetUnPackedDeals(spec)
}

func (m MarketNodeImpl) MarkDealsAsPacking(miner address.Address, deals []abi.DealID) error {
	return m.PieceStore.MarkDealsAsPacking(deals)
}

func (m MarketNodeImpl) UpdateDealOnPacking(miner address.Address, pieceCID cid.Cid, dealId abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error {
	return m.PieceStore.UpdateDealOnPacking(pieceCID, dealId, sectorid, offset)
}

func (m MarketNodeImpl) UpdateDealStatus(miner address.Address, pieceCID cid.Cid, dealId abi.DealID, status string) error {
	return m.PieceStore.UpdateDealStatus(pieceCID, dealId, status)
}

func (m MarketNodeImpl) DealsImportData(ctx context.Context, dealPropCid cid.Cid, fname string) error {
	fi, err := os.Open(fname)
	if err != nil {
		return xerrors.Errorf("failed to open given file: %w", err)
	}
	defer fi.Close() //nolint:errcheck

	return m.StorageProvider.ImportDataForDeal(ctx, dealPropCid, fi)
}

func (m MarketNodeImpl) GetDeals(miner address.Address, pageIndex, pageSize int) ([]*piece.DealInfo, error) {
	return m.PieceStore.GetDeals(pageIndex, pageSize)
}
