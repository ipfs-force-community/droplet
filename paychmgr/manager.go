package paychmgr

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v8/paych"
	"github.com/filecoin-project/venus-market/v2/api/clients"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/metrics"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("paych")

var errProofNotSupported = errors.New("payment channel proof parameter is not supported")

type ChannelAvailableFunds struct {
	// Channel is the address of the channel
	Channel *address.Address
	// From is the from address of the channel (channel creator)
	From address.Address
	// To is the to address of the channel
	To address.Address
	// ConfirmedAmt is the amount of funds that have been confirmed on-chain
	// for the channel
	ConfirmedAmt big.Int
	// PendingAmt is the amount of funds that are pending confirmation on-chain
	PendingAmt big.Int
	// PendingWaitSentinel can be used with PaychGetWaitReady to wait for
	// confirmation of pending funds
	PendingWaitSentinel *cid.Cid
	// QueuedAmt is the amount that is queued up behind a pending request
	QueuedAmt big.Int
	// VoucherRedeemedAmt is the amount that is redeemed by vouchers on-chain
	// and in the local datastore
	VoucherReedeemedAmt big.Int
}

// VoucherCreateResult is the response to calling PaychVoucherCreate
type VoucherCreateResult struct {
	// Voucher that was created, or nil if there was an error or if there
	// were insufficient funds in the channel
	Voucher *paych.SignedVoucher
	// Shortfall is the additional amount that would be needed in the channel
	// in order to be able to create the voucher
	Shortfall big.Int
}

// managerAPI defines all methods needed by the manager
type managerAPI interface {
	IStateManager
	paychDependencyAPI
}

// managerAPIImpl is used to create a composite that implements managerAPI
type managerAPIImpl struct {
	IStateManager
	paychDependencyAPI
}

type Manager struct {
	// The Manager context is used to terminate wait operations on shutdown
	ctx      context.Context
	shutdown context.CancelFunc

	channelInfoRepo repo.PaychChannelInfoRepo
	msgInfoRepo     repo.PaychMsgInfoRepo

	sa     *stateAccessor
	pchapi managerAPI

	lk       sync.RWMutex
	channels map[string]*channelAccessor
}
type ManagerParams struct {
	MPoolAPI     IMessagePush
	ChainInfoAPI IChainInfo
	WalletAPI    IWalletAPI
	SM           IStateManager
}

func NewManager(mctx metrics.MetricsCtx, repo repo.Repo, msgClient clients.IMixMessage, fullNode v1api.FullNode) (*Manager, error) {
	ctx, shutdown := context.WithCancel(mctx)
	impl := &managerAPIImpl{
		IStateManager:      newStateMgrAdapter(fullNode),
		paychDependencyAPI: newPaychDependencyAPI(msgClient, fullNode, fullNode),
	}
	pm := &Manager{
		ctx:             ctx,
		shutdown:        shutdown,
		channelInfoRepo: repo.PaychChannelInfoRepo(),
		msgInfoRepo:     repo.PaychMsgInfoRepo(),
		sa:              &stateAccessor{sm: impl},
		channels:        make(map[string]*channelAccessor),
		pchapi:          impl,
	}
	return pm, pm.Start(ctx)
}

// newManager is used by the tests to supply mocks
func newManager(ctx context.Context, r repo.Repo, pchapi managerAPI) (*Manager, error) {
	var shutdown context.CancelFunc
	ctx, shutdown = context.WithCancel(ctx)
	pm := &Manager{
		sa:              &stateAccessor{sm: pchapi},
		channelInfoRepo: r.PaychChannelInfoRepo(),
		msgInfoRepo:     r.PaychMsgInfoRepo(),
		channels:        make(map[string]*channelAccessor),
		pchapi:          pchapi,
		ctx:             ctx,
		shutdown:        shutdown,
	}
	return pm, pm.Start(ctx)
}

// Start restarts tracking of any messages that were sent to chain.
func (pm *Manager) Start(ctx context.Context) error {
	return pm.restartPending(ctx)
}

// Stop shuts down any processes used by the manager
func (pm *Manager) Stop() {
	pm.shutdown()
}

func (pm *Manager) GetPaych(ctx context.Context, from, to address.Address, amt big.Int) (address.Address, cid.Cid, error) {
	chanAccessor, err := pm.accessorByFromTo(from, to)
	if err != nil {
		return address.Undef, cid.Undef, err
	}

	return chanAccessor.getPaych(ctx, amt)
}

func (pm *Manager) AvailableFunds(ctx context.Context, ch address.Address) (*ChannelAvailableFunds, error) {
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return nil, err
	}

	ci, err := ca.getChannelInfo(ctx, ch)
	if err != nil {
		return nil, err
	}

	return ca.availableFunds(ctx, ci.ChannelID)
}

func (pm *Manager) AvailableFundsByFromTo(ctx context.Context, from address.Address, to address.Address) (*ChannelAvailableFunds, error) {
	ca, err := pm.accessorByFromTo(from, to)
	if err != nil {
		return nil, err
	}

	ci, err := ca.outboundActiveByFromTo(ctx, from, to)
	if errors.Is(err, repo.ErrNotFound) {
		// If there is no active channel between from / to we still want to
		// return an empty ChannelAvailableFunds, so that clients can check
		// for the existence of a channel between from / to without getting
		// an error.
		return &ChannelAvailableFunds{
			Channel:             nil,
			From:                from,
			To:                  to,
			ConfirmedAmt:        big.NewInt(0),
			PendingAmt:          big.NewInt(0),
			PendingWaitSentinel: nil,
			QueuedAmt:           big.NewInt(0),
			VoucherReedeemedAmt: big.NewInt(0),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return ca.availableFunds(ctx, ci.ChannelID)
}

// GetPaychWaitReady waits until the create channel / add funds message with the
// given message CID arrives.
// The returned channel address can safely be used against the Manager methods.
func (pm *Manager) GetPaychWaitReady(ctx context.Context, mcid cid.Cid) (address.Address, error) {
	// Find the channel associated with the message CID
	pm.lk.Lock()
	ci, err := pm.channelInfoRepo.GetChannelByMessageCid(ctx, mcid)
	pm.lk.Unlock()

	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return address.Undef, fmt.Errorf("could not find wait msg cid %s", mcid)
		}
		return address.Undef, err
	}

	chanAccessor, err := pm.accessorByFromTo(ci.Control, ci.Target)
	if err != nil {
		return address.Undef, err
	}

	return chanAccessor.getPaychWaitReady(ctx, mcid)
}

func (pm *Manager) ListChannels(ctx context.Context) ([]address.Address, error) {
	// Need to take an exclusive lock here so that channel operations can't run
	// in parallel (see channelLock)
	pm.lk.Lock()
	defer pm.lk.Unlock()

	return pm.channelInfoRepo.ListChannel(ctx)
}

func (pm *Manager) GetChannelInfo(ctx context.Context, addr address.Address) (*types.ChannelInfo, error) {
	ca, err := pm.accessorByAddress(ctx, addr)
	if err != nil {
		return nil, err
	}
	return ca.getChannelInfo(ctx, addr)
}

func (pm *Manager) CreateVoucher(ctx context.Context, ch address.Address, voucher paych.SignedVoucher) (*VoucherCreateResult, error) {
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return nil, err
	}
	return ca.createVoucher(ctx, ch, voucher)
}

// CheckVoucherValid checks if the given voucher is valid (is or could become spendable at some point).
// If the channel is not in the store, fetches the channel from state (and checks that
// the channel To address is owned by the wallet).
func (pm *Manager) CheckVoucherValid(ctx context.Context, ch address.Address, sv *paych.SignedVoucher) error {
	// Get an accessor for the channel, creating it from state if necessary
	ca, err := pm.inboundChannelAccessor(ctx, ch)
	if err != nil {
		return err
	}

	_, err = ca.checkVoucherValid(ctx, ch, sv)
	return err
}

// CheckVoucherSpendable checks if the given voucher is currently spendable
func (pm *Manager) CheckVoucherSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	if len(proof) > 0 {
		return false, errProofNotSupported
	}
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return false, err
	}

	return ca.checkVoucherSpendable(ctx, ch, sv, secret)
}

// AddVoucherOutbound adds a voucher for an outbound channel.
// Returns an error if the channel is not already in the store.
func (pm *Manager) AddVoucherOutbound(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	if len(proof) > 0 {
		return big.NewInt(0), errProofNotSupported
	}
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return big.NewInt(0), err
	}
	return ca.addVoucher(ctx, ch, sv, minDelta)
}

// AddVoucherInbound adds a voucher for an inbound channel.
// If the channel is not in the store, fetches the channel from state (and checks that
// the channel To address is owned by the wallet).
func (pm *Manager) AddVoucherInbound(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	if len(proof) > 0 {
		return big.NewInt(0), errProofNotSupported
	}
	// Get an accessor for the channel, creating it from state if necessary
	ca, err := pm.inboundChannelAccessor(ctx, ch)
	if err != nil {
		return big.Int{}, err
	}
	return ca.addVoucher(ctx, ch, sv, minDelta)
}

// inboundChannelAccessor gets an accessor for the given channel. The channel
// must either exist in the store, or be an inbound channel that can be created
// from state.
func (pm *Manager) inboundChannelAccessor(ctx context.Context, ch address.Address) (*channelAccessor, error) {
	// Make sure channel is in store, or can be fetched from state, and that
	// the channel To address is owned by the wallet
	ci, err := pm.trackInboundChannel(ctx, ch)
	if err != nil {
		return nil, err
	}

	// This is an inbound channel, so To is the Control address (this node)
	from := ci.Target
	to := ci.Control
	return pm.accessorByFromTo(from, to)
}

func (pm *Manager) trackInboundChannel(ctx context.Context, ch address.Address) (*types.ChannelInfo, error) {
	// Need to take an exclusive lock here so that channel operations can't run
	// in parallel (see channelLock)
	pm.lk.Lock()
	defer pm.lk.Unlock()

	// Check if channel is in store
	ci, err := pm.channelInfoRepo.GetChannelByAddress(ctx, ch)
	if err == nil {
		// Channel is in store, so it's already being tracked
		return ci, nil
	}

	// If there's an error (besides channel not in store) return err
	if !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	// Channel is not in store, so get channel from state
	stateCi, err := pm.sa.loadStateChannelInfo(ctx, ch, types.DirInbound)
	if err != nil {
		return nil, err
	}

	// Check that channel To address is in wallet
	to := stateCi.Control // Inbound channel so To addr is Control (this node)
	toKey, err := pm.pchapi.StateAccountKey(ctx, to, types2.EmptyTSK)
	if err != nil {
		return nil, err
	}
	has, err := pm.pchapi.WalletHas(ctx, toKey)
	if err != nil {
		return nil, err
	}
	if !has {
		msg := "cannot add voucher for channel %s: wallet does not have key for address %s"
		return nil, fmt.Errorf(msg, ch, to)
	}

	// Save channel to store
	return pm.trackChannel(ctx, stateCi)
}

// being tracked
func (pm *Manager) trackChannel(ctx context.Context, ci *types.ChannelInfo) (*types.ChannelInfo, error) {
	_, err := pm.channelInfoRepo.GetChannelByAddress(ctx, *ci.Channel)
	switch err {
	default:
		return nil, err
	case nil:
		return nil, fmt.Errorf("already tracking channel: %s", ci.Channel)
	case repo.ErrNotFound:
		err = pm.channelInfoRepo.SaveChannel(ctx, ci)
		if err != nil {
			return nil, err
		}

		return pm.channelInfoRepo.GetChannelByAddress(ctx, *ci.Channel)
	}
}

// TODO: secret vs proof doesn't make sense, there is only one, not two
func (pm *Manager) SubmitVoucher(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	if len(proof) > 0 {
		return cid.Undef, errProofNotSupported
	}
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return cid.Undef, err
	}
	return ca.submitVoucher(ctx, ch, sv, secret)
}

func (pm *Manager) AllocateLane(ctx context.Context, ch address.Address) (uint64, error) {
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return 0, err
	}
	return ca.allocateLane(ctx, ch)
}

func (pm *Manager) ListVouchers(ctx context.Context, ch address.Address) ([]*types.VoucherInfo, error) {
	ca, err := pm.accessorByAddress(ctx, ch)
	if err != nil {
		return nil, err
	}
	return ca.listVouchers(ctx, ch)
}

func (pm *Manager) Settle(ctx context.Context, addr address.Address) (cid.Cid, error) {
	ca, err := pm.accessorByAddress(ctx, addr)
	if err != nil {
		return cid.Undef, err
	}
	return ca.settle(ctx, addr)
}

func (pm *Manager) Collect(ctx context.Context, addr address.Address) (cid.Cid, error) {
	ca, err := pm.accessorByAddress(ctx, addr)
	if err != nil {
		return cid.Undef, err
	}
	return ca.collect(ctx, addr)
}
