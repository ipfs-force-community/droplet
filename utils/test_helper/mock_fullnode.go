package test_helper

import (
	"context"
	"encoding/json"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/chain"
	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	crypto2 "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/miner"
	paych2 "github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"io"
	"testing"
	"time"
)

type MockFullnode struct {
	*testing.T
}

func (m MockFullnode) DAGGetNode(ctx context.Context, ref string) (interface{}, error) {
	panic("implement me")
}

func (m MockFullnode) DAGGetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	panic("implement me")
}

func (m MockFullnode) DAGCat(ctx context.Context, c cid.Cid) (io.Reader, error) {
	panic("implement me")
}

func (m MockFullnode) DAGImportData(ctx context.Context, data io.Reader) (format.Node, error) {
	panic("implement me")
}

func (m MockFullnode) ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error) {
	panic("implement me")
}

func (m MockFullnode) ChainDeleteObj(ctx context.Context, obj cid.Cid) error {
	panic("implement me")
}

func (m MockFullnode) ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error) {
	panic("implement me")
}

func (m MockFullnode) ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (apitypes.ObjStat, error) {
	panic("implement me")
}

func (m MockFullnode) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) {
	panic("implement me")
}

func (m MockFullnode) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	panic("implement me")
}

func (m MockFullnode) BeaconGetEntry(ctx context.Context, epoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error) {
	panic("implement me")
}

func (m MockFullnode) StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (miner.SectorPreCommitOnChainInfo, error) {
	panic("implement me")
}

func (m MockFullnode) StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorOnChainInfo, error) {
	panic("implement me")
}

func (m MockFullnode) StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorLocation, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (miner.MinerInfo, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]apitypes.Partition, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]apitypes.Deadline, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	panic("implement me")
}

func (m MockFullnode) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*apitypes.MarketDeal, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (chain.CirculatingSupply, error) {
	panic("implement me")
}

func (m MockFullnode) StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error) {
	panic("implement me")
}

func (m MockFullnode) StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]types.MarketDeal, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	panic("implement me")
}

func (m MockFullnode) StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*apitypes.MinerPower, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorExpiration, error) {
	panic("implement me")
}

func (m MockFullnode) StateMinerSectorCount(ctx context.Context, addr address.Address, tsk types.TipSetKey) (apitypes.MinerSectors, error) {
	panic("implement me")
}

func (m MockFullnode) StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (apitypes.MarketBalance, error) {
	panic("implement me")
}

func (m MockFullnode) StateDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, tsk types.TipSetKey) (apitypes.DealCollateralBounds, error) {
	panic("implement me")
}

func (m MockFullnode) StateVerifiedClientStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error) {
	panic("implement me")
}

func (m MockFullnode) BlockTime(ctx context.Context) time.Duration {
	panic("implement me")
}

func (m MockFullnode) ChainList(ctx context.Context, tsKey types.TipSetKey, count int) ([]types.TipSetKey, error) {
	panic("implement me")
}

func (m MockFullnode) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	return []byte("fake token"), 1024, nil
}

func (m MockFullnode) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return MakeTestTipset(m.T), nil
}

func (m MockFullnode) ChainSetHead(ctx context.Context, key types.TipSetKey) error {
	panic("implement me")
}

func (m MockFullnode) ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetTipSetAfterHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetRandomnessFromBeacon(ctx context.Context, key types.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetRandomnessFromTickets(ctx context.Context, tsk types.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	panic("implement me")
}

func (m MockFullnode) StateGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	panic("implement me")
}

func (m MockFullnode) StateGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetBlock(ctx context.Context, id cid.Cid) (*types.BlockHeader, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.UnsignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*apitypes.BlockMessages, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetMessagesInTipset(ctx context.Context, key types.TipSetKey) ([]apitypes.Message, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]apitypes.Message, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error) {
	panic("implement me")
}

func (m MockFullnode) StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error) {
	panic("implement me")
}

func (m MockFullnode) ChainNotify(ctx context.Context) <-chan []*chain.HeadChange {
	panic("implement me")
}

func (m MockFullnode) GetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error) {
	panic("implement me")
}

func (m MockFullnode) GetActor(ctx context.Context, addr address.Address) (*types.Actor, error) {
	panic("implement me")
}

func (m MockFullnode) GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error) {
	panic("implement me")
}

func (m MockFullnode) GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error) {
	panic("implement me")
}

func (m MockFullnode) MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback abi.ChainEpoch) (*chain.ChainMessage, error) {
	panic("implement me")
}

func (m MockFullnode) ProtocolParameters(ctx context.Context) (*apitypes.ProtocolParams, error) {
	panic("implement me")
}

func (m MockFullnode) ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) StateNetworkName(ctx context.Context) (apitypes.NetworkName, error) {
	panic("implement me")
}

func (m MockFullnode) StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	panic("implement me")
}

func (m MockFullnode) StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	panic("implement me")
}

func (m MockFullnode) StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error) {
	panic("implement me")
}

func (m MockFullnode) VerifyEntry(parent, child *types.BeaconEntry, height abi.ChainEpoch) bool {
	panic("implement me")
}

func (m MockFullnode) ChainExport(ctx context.Context, epoch abi.ChainEpoch, b bool, key types.TipSetKey) (<-chan []byte, error) {
	panic("implement me")
}

func (m MockFullnode) ChainGetPath(ctx context.Context, from types.TipSetKey, to types.TipSetKey) ([]*chain.HeadChange, error) {
	panic("implement me")
}

func (m MockFullnode) ConfigSet(ctx context.Context, dottedPath string, paramJSON string) error {
	panic("implement me")
}

func (m MockFullnode) ConfigGet(ctx context.Context, dottedPath string) (interface{}, error) {
	panic("implement me")
}

func (m MockFullnode) StateMarketParticipants(ctx context.Context, tsk types.TipSetKey) (map[string]apitypes.MarketBalance, error) {
	panic("implement me")
}

func (m MockFullnode) MinerGetBaseInfo(ctx context.Context, maddr address.Address, round abi.ChainEpoch, tsk types.TipSetKey) (*apitypes.MiningBaseInfo, error) {
	panic("implement me")
}

func (m MockFullnode) MinerCreateBlock(ctx context.Context, bt *apitypes.BlockTemplate) (*types.BlockMsg, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolDeleteByAdress(ctx context.Context, addr address.Address) error {
	panic("implement me")
}

func (m MockFullnode) MpoolPublishByAddr(ctx context.Context, a address.Address) error {
	panic("implement me")
}

func (m MockFullnode) MpoolPublishMessage(ctx context.Context, smsg *types.SignedMessage) error {
	panic("implement me")
}

func (m MockFullnode) MpoolPush(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolGetConfig(ctx context.Context) (*messagepool.MpoolConfig, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolSetConfig(ctx context.Context, cfg *messagepool.MpoolConfig) error {
	panic("implement me")
}

func (m MockFullnode) MpoolSelect(ctx context.Context, key types.TipSetKey, f float64) ([]*types.SignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolSelects(ctx context.Context, key types.TipSetKey, float64s []float64) ([][]*types.SignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolPending(ctx context.Context, tsk types.TipSetKey) ([]*types.SignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolClear(ctx context.Context, local bool) error {
	panic("implement me")
}

func (m MockFullnode) MpoolPushUntrusted(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec) (*types.SignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolBatchPush(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolBatchPushUntrusted(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolBatchPushMessage(ctx context.Context, msgs []*types.UnsignedMessage, spec *types.MessageSendSpec) ([]*types.SignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolSub(ctx context.Context) (<-chan messagepool.MpoolUpdate, error) {
	panic("implement me")
}

func (m MockFullnode) GasEstimateMessageGas(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec, tsk types.TipSetKey) (*types.UnsignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*types.EstimateMessage, fromNonce uint64, tsk types.TipSetKey) ([]*types.EstimateResult, error) {
	panic("implement me")
}

func (m MockFullnode) GasEstimateFeeCap(ctx context.Context, msg *types.UnsignedMessage, maxqueueblks int64, tsk types.TipSetKey) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk types.TipSetKey) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) GasEstimateGasLimit(ctx context.Context, msgIn *types.UnsignedMessage, tsk types.TipSetKey) (int64, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolCheckMessages(ctx context.Context, protos []*apitypes.MessagePrototype) ([][]apitypes.MessageCheckStatus, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolCheckPendingMessages(ctx context.Context, addr address.Address) ([][]apitypes.MessageCheckStatus, error) {
	panic("implement me")
}

func (m MockFullnode) MpoolCheckReplaceMessages(ctx context.Context, msg []*types.Message) ([][]apitypes.MessageCheckStatus, error) {
	panic("implement me")
}

func (m MockFullnode) MsigCreate(ctx context.Context, req uint64, addrs []address.Address, duration abi.ChainEpoch, val types.BigInt, src address.Address, gp types.BigInt) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigCancel(ctx context.Context, msig address.Address, txID uint64, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (*apitypes.MessagePrototype, error) {
	panic("implement me")
}

func (m MockFullnode) MsigGetVested(ctx context.Context, addr address.Address, start types.TipSetKey, end types.TipSetKey) (types.BigInt, error) {
	panic("implement me")
}

func (m MockFullnode) NetworkGetBandwidthStats(ctx context.Context) metrics.Stats {
	panic("implement me")
}

func (m MockFullnode) NetworkGetPeerAddresses(ctx context.Context) []multiaddr.Multiaddr {
	panic("implement me")
}

func (m MockFullnode) NetworkGetPeerID(ctx context.Context) peer.ID {
	panic("implement me")
}

func (m MockFullnode) NetworkFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo {
	panic("implement me")
}

func (m MockFullnode) NetworkGetClosestPeers(ctx context.Context, key string) (<-chan peer.ID, error) {
	panic("implement me")
}

func (m MockFullnode) NetworkFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error) {
	panic("implement me")
}

func (m MockFullnode) NetworkConnect(ctx context.Context, addrs []string) (<-chan net.ConnectionResult, error) {
	panic("implement me")
}

func (m MockFullnode) NetworkPeers(ctx context.Context, verbose, latency, streams bool) (*net.SwarmConnInfos, error) {
	panic("implement me")
}

func (m MockFullnode) Version(ctx context.Context) (apitypes.Version, error) {
	panic("implement me")
}

func (m MockFullnode) NetAddrsListen(ctx context.Context) (peer.AddrInfo, error) {
	panic("implement me")
}

func (m MockFullnode) PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*apitypes.ChannelInfo, error) {
	panic("implement me")
}

func (m MockFullnode) PaychAvailableFunds(ctx context.Context, ch address.Address) (*paychmgr.ChannelAvailableFunds, error) {
	panic("implement me")
}

func (m MockFullnode) PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*paychmgr.ChannelAvailableFunds, error) {
	panic("implement me")
}

func (m MockFullnode) PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error) {
	panic("implement me")
}

func (m MockFullnode) PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []apitypes.VoucherSpec) (*apitypes.PaymentInfo, error) {
	panic("implement me")
}

func (m MockFullnode) PaychList(ctx context.Context) ([]address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) PaychStatus(ctx context.Context, pch address.Address) (*types.PaychStatus, error) {
	panic("implement me")
}

func (m MockFullnode) PaychSettle(ctx context.Context, addr address.Address) (cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) PaychCollect(ctx context.Context, addr address.Address) (cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) PaychVoucherCheckValid(ctx context.Context, ch address.Address, sv *paych2.SignedVoucher) error {
	panic("implement me")
}

func (m MockFullnode) PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych2.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	panic("implement me")
}

func (m MockFullnode) PaychVoucherAdd(ctx context.Context, ch address.Address, sv *paych2.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*paychmgr.VoucherCreateResult, error) {
	panic("implement me")
}

func (m MockFullnode) PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych2.SignedVoucher, error) {
	panic("implement me")
}

func (m MockFullnode) PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych2.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	panic("implement me")
}

func (m MockFullnode) ChainSyncHandleNewTipSet(ctx context.Context, ci *types.ChainInfo) error {
	panic("implement me")
}

func (m MockFullnode) SetConcurrent(ctx context.Context, concurrent int64) error {
	panic("implement me")
}

func (m MockFullnode) SyncerTracker(ctx context.Context) *syncTypes.TargetTracker {
	panic("implement me")
}

func (m MockFullnode) Concurrent(ctx context.Context) int64 {
	panic("implement me")
}

func (m MockFullnode) ChainTipSetWeight(ctx context.Context, tsk types.TipSetKey) (big.Int, error) {
	panic("implement me")
}

func (m MockFullnode) SyncSubmitBlock(ctx context.Context, blk *types.BlockMsg) error {
	panic("implement me")
}

func (m MockFullnode) StateCall(ctx context.Context, msg *types.UnsignedMessage, tsk types.TipSetKey) (*types.InvocResult, error) {
	panic("implement me")
}

func (m MockFullnode) SyncState(ctx context.Context) (*apitypes.SyncState, error) {
	panic("implement me")
}

func (m MockFullnode) WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto2.Signature, error) {
	signStr := []byte(`{"Type": 1, "Data": "0Te6VibKM4W0E8cgNFZTgiNXzUqgOZJtCPN1DEp2kClTuzUGVzu/umhCM87o76AEpsMkjpJQGo+S8MYHXQdFTAE="}`)
	sign := &crypto.Signature{}
	return sign, json.Unmarshal(signStr, sign)
}

func (m MockFullnode) WalletExport(addr address.Address, password string) (*crypto2.KeyInfo, error) {
	panic("implement me")
}

func (m MockFullnode) WalletImport(key *crypto2.KeyInfo) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	panic("implement me")
}

func (m MockFullnode) WalletNewAddress(protocol address.Protocol) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) {
	panic("implement me")
}

func (m MockFullnode) WalletDefaultAddress(ctx context.Context) (address.Address, error) {
	panic("implement me")
}

func (m MockFullnode) WalletAddresses(ctx context.Context) []address.Address {
	panic("implement me")
}

func (m MockFullnode) WalletSetDefault(ctx context.Context, addr address.Address) error {
	panic("implement me")
}

func (m MockFullnode) WalletSignMessage(ctx context.Context, k address.Address, msg *types.UnsignedMessage) (*types.SignedMessage, error) {
	panic("implement me")
}

func (m MockFullnode) LockWallet(ctx context.Context) error {
	panic("implement me")
}

func (m MockFullnode) UnLockWallet(ctx context.Context, password []byte) error {
	panic("implement me")
}

func (m MockFullnode) SetPassword(Context context.Context, password []byte) error {
	panic("implement me")
}

func (m MockFullnode) HasPassword(Context context.Context) bool {
	panic("implement me")
}

func (m MockFullnode) WalletState(Context context.Context) int {
	panic("implement me")
}

func (m MockFullnode) Verify(ctx context.Context, host, token string) ([]auth.Permission, error) {
	panic("implement me")
}

func (m MockFullnode) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	panic("implement me")
}

var _ apiface.FullNode = (*MockFullnode)(nil)
