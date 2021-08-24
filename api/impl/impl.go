package impl

import (
	"context"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/types"
	mTypes "github.com/filecoin-project/venus-messager/types"
	"github.com/filecoin-project/venus/app/client/apiface"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"os"
	"time"
)

type MarketNodeImpl struct {
	fx.In
	Cfg             *config.MarketConfig
	FullNode        apiface.FullNode
	StorageProvider storagemarket.StorageProvider
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
	panic("implement me")
}

func (m MarketNodeImpl) MarketGetDealUpdates(ctx context.Context) (<-chan storagemarket.MinerDeal, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketListIncompleteDeals(ctx context.Context) ([]storagemarket.MinerDeal, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketSetAsk(ctx context.Context, price vTypes.BigInt, verifiedPrice vTypes.BigInt, duration abi.ChainEpoch, minPieceSize abi.PaddedPieceSize, maxPieceSize abi.PaddedPieceSize) error {
	panic("implement me")
}

func (m MarketNodeImpl) MarketGetAsk(ctx context.Context) (*storagemarket.SignedStorageAsk, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketSetRetrievalAsk(ctx context.Context, rask *retrievalmarket.Ask) error {
	panic("implement me")
}

func (m MarketNodeImpl) MarketGetRetrievalAsk(ctx context.Context) (*retrievalmarket.Ask, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketListDataTransfers(ctx context.Context) ([]types.DataTransferChannel, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketDataTransferUpdates(ctx context.Context) (<-chan types.DataTransferChannel, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketRestartDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) MarketCancelDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) MarketPendingDeals(ctx context.Context) (types.PendingDealInfo, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MarketPublishPendingDeals(ctx context.Context) error {
	panic("implement me")
}

func (m MarketNodeImpl) PiecesListPieces(ctx context.Context) ([]cid.Cid, error) {
	panic("implement me")
}

func (m MarketNodeImpl) PiecesListCidInfos(ctx context.Context) ([]cid.Cid, error) {
	panic("implement me")
}

func (m MarketNodeImpl) PiecesGetPieceInfo(ctx context.Context, pieceCid cid.Cid) (*piecestore.PieceInfo, error) {
	panic("implement me")
}

func (m MarketNodeImpl) PiecesGetCIDInfo(ctx context.Context, payloadCid cid.Cid) (*piecestore.CIDInfo, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsImportData(ctx context.Context, dealPropCid cid.Cid, file string) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsList(ctx context.Context) ([]types.MarketDeal, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsConsiderOnlineStorageDeals(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetConsiderOnlineStorageDeals(ctx context.Context, b bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsConsiderOnlineRetrievalDeals(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetConsiderOnlineRetrievalDeals(ctx context.Context, b bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsPieceCidBlocklist(ctx context.Context) ([]cid.Cid, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetPieceCidBlocklist(ctx context.Context, cids []cid.Cid) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsConsiderOfflineStorageDeals(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetConsiderOfflineStorageDeals(ctx context.Context, b bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsConsiderOfflineRetrievalDeals(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetConsiderOfflineRetrievalDeals(ctx context.Context, b bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsConsiderVerifiedStorageDeals(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetConsiderVerifiedStorageDeals(ctx context.Context, b bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) DealsConsiderUnverifiedStorageDeals(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m MarketNodeImpl) DealsSetConsiderUnverifiedStorageDeals(ctx context.Context, b bool) error {
	panic("implement me")
}

func (m MarketNodeImpl) SectorGetSealDelay(ctx context.Context) (time.Duration, error) {
	panic("implement me")
}

func (m MarketNodeImpl) SectorSetExpectedSealDuration(ctx context.Context, duration time.Duration) error {
	panic("implement me")
}

func (m MarketNodeImpl) MessagerWaitMessage(ctx context.Context, uuid cid.Cid) (*mTypes.Message, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (cid.Cid, error) {
	panic("implement me")
}

func (m MarketNodeImpl) MessagerGetMessage(ctx context.Context, uuid cid.Cid) (*mTypes.Message, error) {
	panic("implement me")
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
