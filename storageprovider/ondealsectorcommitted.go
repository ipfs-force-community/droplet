package storageprovider

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v9/miner"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/events"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	lminer "github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type eventsCalledAPI interface {
	Called(ctx context.Context, check events.CheckFunc, msgHnd events.MsgHandler, rev events.RevertHandler, confidence int, timeout abi.ChainEpoch, mf events.MsgMatchFunc) error
}

type dealInfoAPI interface {
	GetCurrentDealInfo(ctx context.Context, tok types.TipSetKey, proposal *market.DealProposal, publishCid cid.Cid) (CurrentDealInfo, error)
}

type diffPreCommitsAPI interface {
	diffPreCommits(ctx context.Context, actor address.Address, pre, cur types.TipSetKey) (*lminer.PreCommitChanges, error)
}

type SectorCommittedManager struct {
	ev       eventsCalledAPI
	dealInfo dealInfoAPI
	dpc      diffPreCommitsAPI
}

func NewSectorCommittedManager(ev eventsCalledAPI, tskAPI CurrentDealInfoTskAPI, dpcAPI diffPreCommitsAPI) *SectorCommittedManager {
	dim := &CurrentDealInfoManager{
		CDAPI: &CurrentDealInfoAPIAdapter{CurrentDealInfoTskAPI: tskAPI},
	}

	return newSectorCommittedManager(ev, dim, dpcAPI)
}

func newSectorCommittedManager(ev eventsCalledAPI, dealInfo dealInfoAPI, dpcAPI diffPreCommitsAPI) *SectorCommittedManager {
	return &SectorCommittedManager{
		ev:       ev,
		dealInfo: dealInfo,
		dpc:      dpcAPI,
	}
}

func (mgr *SectorCommittedManager) OnDealSectorPreCommitted(ctx context.Context, provider address.Address, proposal market.DealProposal, publishCid cid.Cid, callback storagemarket.DealSectorPreCommittedCallback) error {
	// Ensure callback is only called once
	var once sync.Once
	cb := func(sectorNumber abi.SectorNumber, isActive bool, err error) {
		once.Do(func() {
			callback(sectorNumber, isActive, err)
		})
	}

	// First check if the deal is already active, and if so, bail out
	checkFunc := func(ctx context.Context, ts *types.TipSet) (done bool, more bool, err error) {
		dealInfo, isActive, err := mgr.checkIfDealAlreadyActive(ctx, ts, &proposal, publishCid)
		if err != nil {
			// Note: the error returned from here will end up being returned
			// from OnDealSectorPreCommitted so no need to call the callback
			// with the error
			return false, false, fmt.Errorf("failed to check deal activity: %w", err)
		}

		if isActive {
			// Deal is already active, bail out
			cb(0, true, nil)
			return true, false, nil
		}

		// Check that precommits which landed between when the deal was published
		// and now don't already contain the deal we care about.
		// (this can happen when the precommit lands vary quickly (in tests), or
		// when the client node was down after the deal was published, and when
		// the precommit containing it landed on chain)

		diff, err := mgr.dpc.diffPreCommits(ctx, provider, dealInfo.PublishMsgTipSet, ts.Key())
		if err != nil {
			return false, false, fmt.Errorf("failed to diff precommits: %w", err)
		}

		for _, info := range diff.Added {
			for _, d := range info.Info.DealIDs {
				if d == dealInfo.DealID {
					cb(info.Info.SectorNumber, false, nil)
					return true, false, nil
				}
			}
		}

		// Not yet active, start matching against incoming messages
		return false, true, nil
	}

	// Watch for a pre-commit message to the provider.
	matchEvent := func(msg *types.Message) (bool, error) {
		matched := msg.To == provider && (msg.Method == builtin.MethodsMiner.PreCommitSector || msg.Method == builtin.MethodsMiner.PreCommitSectorBatch)
		return matched, nil
	}

	// The deal must be accepted by the deal proposal start epoch, so timeout
	// if the chain reaches that epoch
	timeoutEpoch := proposal.StartEpoch + 1

	// Check if the message params included the deal ID we're looking for.
	called := func(msg *types.Message, rec *types.MessageReceipt, ts *types.TipSet, curH abi.ChainEpoch) (more bool, err error) {
		defer func() {
			if err != nil {
				cb(0, false, fmt.Errorf("handling applied event: %w", err))
			}
		}()

		// If the deal hasn't been activated by the proposed start epoch, the
		// deal will timeout (when msg == nil it means the timeout epoch was reached)
		if msg == nil {
			err = fmt.Errorf("deal with piece CID %s was not activated by proposed deal start epoch %d", proposal.PieceCID, proposal.StartEpoch)
			return false, err
		}

		// Ignore the pre-commit message if it was not executed successfully
		if rec.ExitCode != 0 {
			return true, nil
		}

		// When there is a reorg, the deal ID may change, so get the
		// current deal ID from the publish message CID
		res, err := mgr.dealInfo.GetCurrentDealInfo(ctx, ts.Key(), &proposal, publishCid)
		if err != nil {
			return false, fmt.Errorf("failed to get dealinfo: %w", err)
		}

		// If this is a replica update method that succeeded the deal is active
		if msg.Method == builtin.MethodsMiner.ProveReplicaUpdates {
			sn, err := dealSectorInReplicaUpdateSuccess(msg, rec, res)
			if err != nil {
				return false, err
			}
			if sn != nil {
				cb(*sn, true, nil)
				return false, nil
			}
			// Didn't find the deal ID in this message, so keep looking
			return true, nil
		}

		// Extract the message parameters
		sn, err := dealSectorInPreCommitMsg(msg, res)
		if err != nil {
			return false, fmt.Errorf("failed to extract message params: %w", err)
		}

		if sn != nil {
			cb(*sn, false, nil)
		}

		// Didn't find the deal ID in this message, so keep looking
		return true, nil
	}

	revert := func(ctx context.Context, ts *types.TipSet) error {
		log.Warn("deal pre-commit reverted; TODO: actually handle this!")
		// TODO: Just go back to DealSealing?
		return nil
	}

	if err := mgr.ev.Called(ctx, checkFunc, called, revert, int(constants.MessageConfidence+1), timeoutEpoch, matchEvent); err != nil {
		return fmt.Errorf("failed to set up called handler: %w", err)
	}

	return nil
}

func (mgr *SectorCommittedManager) OnDealSectorCommitted(ctx context.Context, provider address.Address, sectorNumber abi.SectorNumber, proposal market.DealProposal, publishCid cid.Cid, callback storagemarket.DealSectorCommittedCallback) error {
	// Ensure callback is only called once
	var once sync.Once
	cb := func(err error) {
		once.Do(func() {
			callback(err)
		})
	}

	// First check if the deal is already active, and if so, bail out
	checkFunc := func(ctx context.Context, ts *types.TipSet) (done bool, more bool, err error) {
		_, isActive, err := mgr.checkIfDealAlreadyActive(ctx, ts, &proposal, publishCid)
		if err != nil {
			// Note: the error returned from here will end up being returned
			// from OnDealSectorCommitted so no need to call the callback
			// with the error
			return false, false, err
		}

		if isActive {
			// Deal is already active, bail out
			cb(nil)
			return true, false, nil
		}

		// Not yet active, start matching against incoming messages
		return false, true, nil
	}

	// Match a prove-commit sent to the provider with the given sector number
	matchEvent := func(msg *types.Message) (matched bool, err error) {
		if msg.To != provider {
			return false, nil
		}

		return sectorInCommitMsg(msg, sectorNumber)
	}

	// The deal must be accepted by the deal proposal start epoch, so timeout
	// if the chain reaches that epoch
	timeoutEpoch := proposal.StartEpoch + 1

	called := func(msg *types.Message, rec *types.MessageReceipt, ts *types.TipSet, curH abi.ChainEpoch) (more bool, err error) {
		defer func() {
			if err != nil {
				cb(fmt.Errorf("handling applied event: %w", err))
			}
		}()

		// If the deal hasn't been activated by the proposed start epoch, the
		// deal will timeout (when msg == nil it means the timeout epoch was reached)
		if msg == nil {
			err := fmt.Errorf("deal with piece CID %s was not activated by proposed deal start epoch %d", proposal.PieceCID, proposal.StartEpoch)
			return false, err
		}

		// Ignore the prove-commit message if it was not executed successfully
		if rec.ExitCode != 0 {
			return true, nil
		}

		// Get the deal info
		res, err := mgr.dealInfo.GetCurrentDealInfo(ctx, ts.Key(), &proposal, publishCid)
		if err != nil {
			return false, fmt.Errorf("failed to look up deal on chain: %w", err)
		}

		// Make sure the deal is active
		if res.MarketDeal.State.SectorStartEpoch < 1 {
			return false, fmt.Errorf("deal wasn't active: deal=%d, parentState=%s, h=%d", res.DealID, ts.Parents(), ts.Height())
		}

		log.Infof("Storage deal %d activated at epoch %d", res.DealID, res.MarketDeal.State.SectorStartEpoch)

		cb(nil)

		return false, nil
	}

	revert := func(ctx context.Context, ts *types.TipSet) error {
		log.Warn("deal activation reverted; TODO: actually handle this!")
		// TODO: Just go back to DealSealing?
		return nil
	}

	if err := mgr.ev.Called(ctx, checkFunc, called, revert, int(constants.MessageConfidence+1), timeoutEpoch, matchEvent); err != nil {
		return fmt.Errorf("failed to set up called handler: %w", err)
	}

	return nil
}

func dealSectorInReplicaUpdateSuccess(msg *types.Message, rec *types.MessageReceipt, res CurrentDealInfo) (*abi.SectorNumber, error) {
	var params miner.ProveReplicaUpdatesParams
	if err := params.UnmarshalCBOR(bytes.NewReader(msg.Params)); err != nil {
		return nil, fmt.Errorf("unmarshal prove replica update: %w", err)
	}

	var seekUpdate miner.ReplicaUpdate
	var found bool
	for _, update := range params.Updates {
		for _, did := range update.Deals {
			if did == res.DealID {
				seekUpdate = update
				found = true
				break
			}
		}
	}
	if !found {
		return nil, nil
	}

	// check that this update passed validation steps
	var successBf bitfield.BitField
	if err := successBf.UnmarshalCBOR(bytes.NewReader(rec.Return)); err != nil {
		return nil, fmt.Errorf("unmarshal return value: %w", err)
	}
	success, err := successBf.IsSet(uint64(seekUpdate.SectorID))
	if err != nil {
		return nil, fmt.Errorf("failed to check success of replica update: %w", err)
	}
	if !success {
		return nil, fmt.Errorf("replica update %d failed", seekUpdate.SectorID)
	}
	return &seekUpdate.SectorID, nil
}

// dealSectorInPreCommitMsg tries to find a sector containing the specified deal
func dealSectorInPreCommitMsg(msg *types.Message, res CurrentDealInfo) (*abi.SectorNumber, error) {
	switch msg.Method {
	case builtin.MethodsMiner.PreCommitSector:
		var params miner.SectorPreCommitInfo
		if err := params.UnmarshalCBOR(bytes.NewReader(msg.Params)); err != nil {
			return nil, fmt.Errorf("unmarshal pre commit: %w", err)
		}

		// Check through the deal IDs associated with this message
		for _, did := range params.DealIDs {
			if did == res.DealID {
				// Found the deal ID in this message. Callback with the sector ID.
				return &params.SectorNumber, nil
			}
		}
	case builtin.MethodsMiner.PreCommitSectorBatch:
		var params miner.PreCommitSectorBatchParams
		if err := params.UnmarshalCBOR(bytes.NewReader(msg.Params)); err != nil {
			return nil, fmt.Errorf("unmarshal pre commit: %w", err)
		}

		for _, precommit := range params.Sectors {
			// Check through the deal IDs associated with this message
			for _, did := range precommit.DealIDs {
				if did == res.DealID {
					// Found the deal ID in this message. Callback with the sector ID.
					return &precommit.SectorNumber, nil
				}
			}
		}
	default:
		return nil, fmt.Errorf("unexpected method %d", msg.Method)
	}

	return nil, nil
}

// sectorInCommitMsg checks if the provided message commits specified sector
func sectorInCommitMsg(msg *types.Message, sectorNumber abi.SectorNumber) (bool, error) {
	switch msg.Method {
	case builtin.MethodsMiner.ProveCommitSector:
		var params miner.ProveCommitSectorParams
		if err := params.UnmarshalCBOR(bytes.NewReader(msg.Params)); err != nil {
			return false, fmt.Errorf("failed to unmarshal prove commit sector params: %w", err)
		}

		return params.SectorNumber == sectorNumber, nil

	case builtin.MethodsMiner.ProveCommitAggregate:
		var params miner.ProveCommitAggregateParams
		if err := params.UnmarshalCBOR(bytes.NewReader(msg.Params)); err != nil {
			return false, fmt.Errorf("failed to unmarshal prove commit sector params: %w", err)
		}

		set, err := params.SectorNumbers.IsSet(uint64(sectorNumber))
		if err != nil {
			return false, fmt.Errorf("checking if sectorNumber is set in commit aggregate message: %w", err)
		}

		return set, nil

	default:
		return false, nil
	}
}

func (mgr *SectorCommittedManager) checkIfDealAlreadyActive(ctx context.Context, ts *types.TipSet, proposal *market.DealProposal, publishCid cid.Cid) (CurrentDealInfo, bool, error) {
	res, err := mgr.dealInfo.GetCurrentDealInfo(ctx, ts.Key(), proposal, publishCid)
	if err != nil {
		// TODO: This may be fine for some errors
		return res, false, fmt.Errorf("failed to look up deal on chain: %w", err)
	}

	// Sector was slashed
	if res.MarketDeal.State.SlashEpoch > 0 {
		return res, false, fmt.Errorf("deal %d was slashed at epoch %d", res.DealID, res.MarketDeal.State.SlashEpoch)
	}

	// Sector with deal is already active
	if res.MarketDeal.State.SectorStartEpoch > 0 {
		return res, true, nil
	}

	return res, false, nil
}
