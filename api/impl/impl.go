package impl

import (
	"context"
	"fmt"
	"github.com/filecoin-project/venus-market/models/repo"
	"os"
	"sort"
	"time"

	"github.com/filecoin-project/specs-actors/actors/builtin/paych"

	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/retrievalprovider"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus-market/api"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/piecestorage"
	storageadapter2 "github.com/filecoin-project/venus-market/storageprovider"
	"github.com/filecoin-project/venus-market/types"

	"github.com/filecoin-project/venus-market/paychmgr"
	mTypes "github.com/filecoin-project/venus-messager/types"

	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/constants"
	vTypes "github.com/filecoin-project/venus/pkg/types"
)

var _ api.MarketFullNode = (*MarketNodeImpl)(nil)
var log = logging.Logger("market_api")

type MarketNodeImpl struct {
	FundAPI
	MarketEventAPI
	fx.In

	FullNode            apiface.FullNode
	Host                host.Host
	StorageProvider     storageadapter2.StorageProviderV2
	RetrievalProvider   retrievalprovider.IRetrievalProvider
	RetrievalAskHandler retrievalprovider.IAskHandler
	DataTransfer        network.ProviderDataTransfer
	DealPublisher       *storageadapter2.DealPublisher
	DealAssigner        storageadapter2.DealAssiger

	Messager                                    clients2.IMessager `optional:"true"`
	DAGStore                                    *dagstore.DAGStore
	PieceStorage                                piecestorage.IPieceStorage
	MinerMgr                                    minermgr.IMinerMgr
	PaychAPI                                    *paychmgr.PaychAPI
	Repo                                        repo.Repo
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

func (m MarketNodeImpl) ActorAddress(ctx context.Context) ([]address.Address, error) {
	return m.MinerMgr.ActorAddress(ctx)
}

func (m MarketNodeImpl) ActorExist(ctx context.Context, addr address.Address) (bool, error) {
	return m.MinerMgr.Has(ctx, addr), nil
}

func (m MarketNodeImpl) ActorSectorSize(ctx context.Context, addr address.Address) (abi.SectorSize, error) {
	if bHas := m.MinerMgr.Has(ctx, addr); bHas {
		minerInfo, err := m.FullNode.StateMinerInfo(ctx, addr, vTypes.EmptyTSK)
		if err != nil {
			return 0, err
		}

		return minerInfo.SectorSize, nil
	}

	return 0, xerrors.New("not found")
}

func (m MarketNodeImpl) MarketImportDealData(ctx context.Context, propCid cid.Cid, path string) error {
	fi, err := os.Open(path)
	if err != nil {
		return xerrors.Errorf("failed to open file: %w", err)
	}
	defer fi.Close() //nolint:errcheck

	return m.StorageProvider.ImportDataForDeal(ctx, propCid, fi)
}

func (m MarketNodeImpl) MarketListDeals(ctx context.Context, addrs []address.Address) ([]types.MarketDeal, error) {
	return m.listDeals(ctx, addrs)
}

func (m MarketNodeImpl) MarketListRetrievalDeals(ctx context.Context, mAddr address.Address) ([]retrievalmarket.ProviderDealState, error) {
	var out []retrievalmarket.ProviderDealState
	deals, err := m.RetrievalProvider.ListDeals()
	if err != nil {
		return nil, err
	}

	for _, deal := range deals {
		if deal.ChannelID != nil {
			if deal.ChannelID.Initiator == "" || deal.ChannelID.Responder == "" {
				deal.ChannelID = nil // don't try to push unparsable peer IDs over jsonrpc
			}
		}
		// todo: 按miner过滤交易
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

func (m MarketNodeImpl) MarketListIncompleteDeals(ctx context.Context, mAddr address.Address) ([]storagemarket.MinerDeal, error) {
	return m.StorageProvider.ListLocalDeals(mAddr)
}

func (m MarketNodeImpl) MarketSetAsk(ctx context.Context, mAddr address.Address, price vTypes.BigInt, verifiedPrice vTypes.BigInt, duration abi.ChainEpoch, minPieceSize abi.PaddedPieceSize, maxPieceSize abi.PaddedPieceSize) error {
	options := []storagemarket.StorageAskOption{
		storagemarket.MinPieceSize(minPieceSize),
		storagemarket.MaxPieceSize(maxPieceSize),
	}

	return m.StorageProvider.SetAsk(mAddr, price, verifiedPrice, duration, options...)
}

func (m MarketNodeImpl) MarketGetAsk(ctx context.Context, mAddr address.Address) (*storagemarket.SignedStorageAsk, error) {
	return m.StorageProvider.GetAsk(mAddr)
}

func (m MarketNodeImpl) MarketSetRetrievalAsk(ctx context.Context, mAddr address.Address, ask *retrievalmarket.Ask) error {
	return m.RetrievalAskHandler.SetAsk(mAddr, ask)
}

func (m MarketNodeImpl) MarketGetRetrievalAsk(ctx context.Context, mAddr address.Address) (*retrievalmarket.Ask, error) {
	return m.RetrievalAskHandler.GetAsk(mAddr)
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
	return m.Repo.StorageDealRepo().ListPieceInfoKeys()
}

func (m MarketNodeImpl) PiecesListCidInfos(ctx context.Context) ([]cid.Cid, error) {
	return m.Repo.CidInfoRepo().ListCidInfoKeys()
}

func (m MarketNodeImpl) PiecesGetPieceInfo(ctx context.Context, pieceCid cid.Cid) (*piecestore.PieceInfo, error) {
	pi, err := m.Repo.StorageDealRepo().GetPieceInfo(pieceCid)
	if err != nil {
		return nil, err
	}
	return pi, nil
}

func (m MarketNodeImpl) PiecesGetCIDInfo(ctx context.Context, payloadCid cid.Cid) (*piecestore.CIDInfo, error) {
	ci, err := m.Repo.CidInfoRepo().GetCIDInfo(payloadCid)
	if err != nil {
		return nil, err
	}

	return &ci, nil
}

func (m MarketNodeImpl) DealsConsiderOnlineStorageDeals(ctx context.Context) (bool, error) {
	return m.ConsiderOnlineStorageDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderOnlineStorageDeals(ctx context.Context, b bool) error {
	return m.SetConsiderOnlineStorageDealsConfigFunc(b)
}

func (m MarketNodeImpl) DealsConsiderOnlineRetrievalDeals(ctx context.Context) (bool, error) {
	return m.ConsiderOnlineRetrievalDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderOnlineRetrievalDeals(ctx context.Context, b bool) error {
	return m.SetConsiderOnlineRetrievalDealsConfigFunc(b)
}

func (m MarketNodeImpl) DealsPieceCidBlocklist(ctx context.Context) ([]cid.Cid, error) {
	return m.StorageDealPieceCidBlocklistConfigFunc()
}

func (m MarketNodeImpl) DealsSetPieceCidBlocklist(ctx context.Context, cids []cid.Cid) error {
	return m.SetStorageDealPieceCidBlocklistConfigFunc(cids)
}

func (m MarketNodeImpl) DealsConsiderOfflineStorageDeals(ctx context.Context) (bool, error) {
	return m.ConsiderOfflineStorageDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderOfflineStorageDeals(ctx context.Context, b bool) error {
	return m.SetConsiderOfflineStorageDealsConfigFunc(b)
}

func (m MarketNodeImpl) DealsConsiderOfflineRetrievalDeals(ctx context.Context) (bool, error) {
	return m.ConsiderOfflineRetrievalDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderOfflineRetrievalDeals(ctx context.Context, b bool) error {
	return m.SetConsiderOfflineRetrievalDealsConfigFunc(b)
}

func (m MarketNodeImpl) DealsConsiderVerifiedStorageDeals(ctx context.Context) (bool, error) {
	return m.ConsiderVerifiedStorageDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderVerifiedStorageDeals(ctx context.Context, b bool) error {
	return m.SetConsiderVerifiedStorageDealsConfigFunc(b)
}

func (m MarketNodeImpl) DealsConsiderUnverifiedStorageDeals(ctx context.Context) (bool, error) {
	return m.ConsiderUnverifiedStorageDealsConfigFunc()
}

func (m MarketNodeImpl) DealsSetConsiderUnverifiedStorageDeals(ctx context.Context, b bool) error {
	return m.SetConsiderUnverifiedStorageDealsConfigFunc(b)
}

func (m MarketNodeImpl) SectorGetSealDelay(ctx context.Context) (time.Duration, error) {
	return m.GetExpectedSealDurationFunc()
}

func (m MarketNodeImpl) SectorSetExpectedSealDuration(ctx context.Context, duration time.Duration) error {
	return m.SetExpectedSealDurationFunc(duration)
}

func (m MarketNodeImpl) MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*apitypes.MsgLookup, error) {
	//StateWaitMsg method has been replace in messager mode
	return m.FullNode.StateWaitMsg(ctx, mid, constants.MessageConfidence, constants.LookbackNoLimit, false)
}

func (m MarketNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (*vTypes.SignedMessage, error) {
	//MpoolPushMessage method has been replace in messager mode
	var spec *vTypes.MessageSendSpec
	if meta != nil {
		spec = &vTypes.MessageSendSpec{
			MaxFee:            meta.MaxFee,
			GasOverEstimation: meta.GasOverEstimation,
		}
	}
	return m.FullNode.MpoolPushMessage(ctx, msg, spec)
}

func (m MarketNodeImpl) MessagerGetMessage(ctx context.Context, mid cid.Cid) (*vTypes.Message, error) {
	//ChainGetMessage method has been replace in messager mode
	return m.FullNode.ChainGetMessage(ctx, mid)
}

func (m MarketNodeImpl) listDeals(ctx context.Context, addrs []address.Address) ([]types.MarketDeal, error) {
	ts, err := m.FullNode.ChainHead(ctx)
	if err != nil {
		return nil, err
	}

	allDeals, err := m.FullNode.StateMarketDeals(ctx, ts.Key())
	if err != nil {
		return nil, err
	}

	var out []types.MarketDeal

	has := func(addr address.Address) bool {
		for _, a := range addrs {
			if a == addr {
				return true
			}
		}

		return false
	}

	for _, deal := range allDeals {
		if m.MinerMgr.Has(ctx, deal.Proposal.Provider) && has(deal.Proposal.Provider) {
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

func (m MarketNodeImpl) DagstoreListShards(ctx context.Context) ([]types.DagstoreShardInfo, error) {
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

func (m MarketNodeImpl) DagstoreInitializeShard(ctx context.Context, key string) error {
	k := shard.KeyFromString(key)

	info, err := m.DAGStore.GetShardInfo(k)
	if err != nil {
		return fmt.Errorf("failed to get shard info: %w", err)
	}
	if st := info.ShardState; st != dagstore.ShardStateNew {
		return fmt.Errorf("cannot initialize shard; expected state ShardStateNew, was: %s", st.String())
	}

	ch := make(chan dagstore.ShardResult, 1)
	if err = m.DAGStore.AcquireShard(ctx, k, ch, dagstore.AcquireOpts{}); err != nil {
		return fmt.Errorf("failed to acquire shard: %w", err)
	}

	var res dagstore.ShardResult
	select {
	case res = <-ch:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := res.Error; err != nil {
		return fmt.Errorf("failed to acquire shard: %w", err)
	}

	if res.Accessor != nil {
		err = res.Accessor.Close()
		if err != nil {
			log.Warnw("failed to close shard accessor; continuing", "shard_key", k, "error", err)
		}
	}

	return nil
}

func (m MarketNodeImpl) DagstoreInitializeAll(ctx context.Context, params types.DagstoreInitializeAllParams) (<-chan types.DagstoreInitializeAllEvent, error) {
	// prepare the thottler tokens.
	var throttle chan struct{}
	if c := params.MaxConcurrency; c > 0 {
		throttle = make(chan struct{}, c)
		for i := 0; i < c; i++ {
			throttle <- struct{}{}
		}
	}

	// are we initializing only unsealed pieces?
	onlyUnsealed := !params.IncludeSealed

	info := m.DAGStore.AllShardsInfo()
	var toInitialize []string
	for k, i := range info {
		if i.ShardState != dagstore.ShardStateNew {
			continue
		}

		// if we're initializing only unsealed pieces, check if there's an
		// unsealed deal for this piece available.
		if onlyUnsealed {
			pieceCid, err := cid.Decode(k.String())
			if err != nil {
				log.Warnw("DagstoreInitializeAll: failed to decode shard key as piece CID; skipping", "shard_key", k.String(), "error", err)
				continue
			}

			isUnsealed, err := m.PieceStorage.Has(pieceCid.String())
			if err != nil {
				log.Warnw("DagstoreInitializeAll: failed to get unsealed status; skipping deal", "piece cid", pieceCid, "error", err)
				continue
			}

			if !isUnsealed {
				log.Infow("DagstoreInitializeAll: skipping piece because it's sealed", "piece_cid", pieceCid, "error", err)
				continue
			}
		}

		// yes, we're initializing this shard.
		toInitialize = append(toInitialize, k.String())
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

func (m MarketNodeImpl) DagstoreRecoverShard(ctx context.Context, key string) error {

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

func (m MarketNodeImpl) DagstoreGC(ctx context.Context) ([]types.DagstoreShardResult, error) {
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

func (m MarketNodeImpl) GetUnPackedDeals(ctx context.Context, miner address.Address, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error) {
	return m.DealAssigner.GetUnPackedDeals(ctx, miner, spec)
}

func (m MarketNodeImpl) AssignUnPackedDeals(ctx context.Context, miner address.Address, ssize abi.SectorSize, spec *types.GetDealSpec) ([]*types.DealInfoIncludePath, error) {
	return m.DealAssigner.AssignUnPackedDeals(ctx, miner, ssize, spec)
}

func (m MarketNodeImpl) MarkDealsAsPacking(ctx context.Context, miner address.Address, deals []abi.DealID) error {
	return m.DealAssigner.MarkDealsAsPacking(ctx, miner, deals)
}

func (m MarketNodeImpl) UpdateDealOnPacking(ctx context.Context, miner address.Address, dealId abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error {
	return m.DealAssigner.UpdateDealOnPacking(ctx, miner, dealId, sectorid, offset)
}

func (m MarketNodeImpl) UpdateDealStatus(ctx context.Context, miner address.Address, dealId abi.DealID, status string) error {
	return m.DealAssigner.UpdateDealStatus(ctx, miner, dealId, status)
}

func (m MarketNodeImpl) DealsImportData(ctx context.Context, dealPropCid cid.Cid, fname string) error {
	fi, err := os.Open(fname)
	if err != nil {
		return xerrors.Errorf("failed to open given file: %w", err)
	}
	defer fi.Close() //nolint:errcheck

	return m.StorageProvider.ImportDataForDeal(ctx, dealPropCid, fi)
}

func (m MarketNodeImpl) GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*types.DealInfo, error) {
	return m.DealAssigner.GetDeals(ctx, miner, pageIndex, pageSize)
}

func (m MarketNodeImpl) PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error) {
	return m.PaychAPI.PaychVoucherList(ctx, pch)
}
