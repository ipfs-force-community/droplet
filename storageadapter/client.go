package storageadapter

// this file implements storagemarket.StorageClientNode

import (
	"bytes"
	"context"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/constants"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/wallet"

	"github.com/ipfs/go-cid"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"

	miner2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	market2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/market"

	"github.com/filecoin-project/venus-market/fundmgr"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/utils"
	vcrypto "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/events"
	"github.com/filecoin-project/venus/pkg/events/state"
	"github.com/filecoin-project/venus/pkg/types"
	marketactor "github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
)

type ClientNodeAdapter struct {
	*clientApi

	fundmgr   *fundmgr.FundManager
	ev        *events.Events
	dsMatcher *dealStateMatcher
	scMgr     *SectorCommittedManager
	cfg       *config.MarketClientConfig
}

type clientApi struct {
	full          apiface.FullNode
	singerService clients2.ISinger
}

func NewClientNodeAdapter(mctx metrics.MetricsCtx, lc fx.Lifecycle, fullNode apiface.FullNode, singerService clients2.ISinger, fundmgr *fundmgr.FundManager, cfg *config.MarketClientConfig) storagemarket.StorageClientNode {
	capi := &clientApi{fullNode, singerService}
	ctx := metrics.LifecycleCtx(mctx, lc)

	ev, err := events.NewEvents(ctx, capi.full)
	if err != nil {
		//todo add error return?
		log.Fatal(err)
	}
	a := &ClientNodeAdapter{
		clientApi: capi,

		fundmgr:   fundmgr,
		ev:        ev,
		cfg:       cfg,
		dsMatcher: newDealStateMatcher(state.NewStatePredicates(state.WrapFastAPI(capi.full))),
	}
	a.scMgr = NewSectorCommittedManager(ev, a.full, &apiWrapper{api: capi.full})
	return a
}

func (c *ClientNodeAdapter) ListStorageProviders(ctx context.Context, encodedTs shared.TipSetToken) ([]*storagemarket.StorageProviderInfo, error) {
	tsk, err := types.TipSetKeyFromBytes(encodedTs)
	if err != nil {
		return nil, err
	}

	addresses, err := c.full.StateListMiners(ctx, tsk)
	if err != nil {
		return nil, err
	}

	var out []*storagemarket.StorageProviderInfo

	for _, addr := range addresses {
		mi, err := c.GetMinerInfo(ctx, addr, encodedTs)
		if err != nil {
			return nil, err
		}

		out = append(out, mi)
	}

	return out, nil
}

func (c *ClientNodeAdapter) VerifySignature(ctx context.Context, sig crypto.Signature, addr address.Address, input []byte, encodedTs shared.TipSetToken) (bool, error) {
	addr, err := c.full.StateAccountKey(ctx, addr, types.EmptyTSK)
	if err != nil {
		return false, err
	}

	err = vcrypto.Verify(&sig, addr, input)
	return err == nil, err
}

// Adds funds with the StorageMinerActor for a piecestorage participant.  Used by both providers and clients.
func (c *ClientNodeAdapter) AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	// (Provider Node API)
	smsg, err := c.full.MpoolPushMessage(ctx, &types.Message{ //todo send to messager service
		To:     miner2.StorageMarketActorAddr,
		From:   addr,
		Value:  amount,
		Method: miner2.MethodsMarket.AddBalance,
	}, nil)
	if err != nil {
		return cid.Undef, err
	}

	return smsg.Cid(), nil
}

func (c *ClientNodeAdapter) ReserveFunds(ctx context.Context, wallet, addr address.Address, amt abi.TokenAmount) (cid.Cid, error) {
	return c.fundmgr.Reserve(ctx, wallet, addr, amt)
}

func (c *ClientNodeAdapter) ReleaseFunds(ctx context.Context, addr address.Address, amt abi.TokenAmount) error {
	return c.fundmgr.Release(addr, amt)
}

func (c *ClientNodeAdapter) GetBalance(ctx context.Context, addr address.Address, encodedTs shared.TipSetToken) (storagemarket.Balance, error) {
	tsk, err := types.TipSetKeyFromBytes(encodedTs)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	bal, err := c.full.StateMarketBalance(ctx, addr, tsk)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	return utils.ToSharedBalance(bal), nil
}

// ValidatePublishedDeal validates that the provided deal has appeared on chain and references the same ClientDeal
// returns the Deal id if there is no error
// TODO: Don't return deal ID
func (c *ClientNodeAdapter) ValidatePublishedDeal(ctx context.Context, deal storagemarket.ClientDeal) (abi.DealID, error) {
	log.Infow("DEAL ACCEPTED!")

	pubmsg, err := c.full.ChainGetMessage(ctx, *deal.PublishMessage)
	if err != nil {
		return 0, xerrors.Errorf("getting deal publish message: %w", err)
	}

	mi, err := c.full.StateMinerInfo(ctx, deal.Proposal.Provider, types.EmptyTSK)
	if err != nil {
		return 0, xerrors.Errorf("getting miner worker failed: %w", err)
	}

	fromid, err := c.full.StateLookupID(ctx, pubmsg.From, types.EmptyTSK)
	if err != nil {
		return 0, xerrors.Errorf("failed to resolve from msg ID addr: %w", err)
	}

	var pubOk bool
	pubAddrs := append([]address.Address{mi.Worker, mi.Owner}, mi.ControlAddresses...)
	for _, a := range pubAddrs {
		if fromid == a {
			pubOk = true
			break
		}
	}
	if !pubOk {
		return 0, xerrors.Errorf("deal wasn't published by piecestorage provider: from=%s, provider=%s,%+v", pubmsg.From, deal.Proposal.Provider, pubAddrs)
	}

	if pubmsg.To != miner2.StorageMarketActorAddr {
		return 0, xerrors.Errorf("deal publish message wasn't set to StorageMarket actor (to=%s)", pubmsg.To)
	}

	if pubmsg.Method != miner2.MethodsMarket.PublishStorageDeals {
		return 0, xerrors.Errorf("deal publish message called incorrect method (method=%s)", pubmsg.Method)
	}

	var params market2.PublishStorageDealsParams
	if err := params.UnmarshalCBOR(bytes.NewReader(pubmsg.Params)); err != nil {
		return 0, err
	}

	dealIdx := -1
	for i, storageDeal := range params.Deals {
		// TODO: make it less hacky
		sd := storageDeal
		eq, err := cborutil.Equals(&deal.ClientDealProposal, &sd)
		if err != nil {
			return 0, err
		}
		if eq {
			dealIdx = i
			break
		}
	}

	if dealIdx == -1 {
		return 0, xerrors.Errorf("deal publish didn't contain our deal (message cid: %s)", deal.PublishMessage)
	}

	// TODO: timeout
	ret, err := c.full.StateWaitMsg(ctx, *deal.PublishMessage, constants.MessageConfidence, constants.LookbackNoLimit, true)
	if err != nil {
		return 0, xerrors.Errorf("waiting for deal publish message: %w", err)
	}
	if ret.Receipt.ExitCode != 0 {
		return 0, xerrors.Errorf("deal publish failed: exit=%d", ret.Receipt.ExitCode)
	}

	var res market2.PublishStorageDealsReturn
	if err := res.UnmarshalCBOR(bytes.NewReader(ret.Receipt.ReturnValue)); err != nil {
		return 0, err
	}

	return res.IDs[dealIdx], nil
}

var clientOverestimation = struct {
	numerator   int64
	denominator int64
}{
	numerator:   12,
	denominator: 10,
}

func (c *ClientNodeAdapter) DealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, isVerified bool) (abi.TokenAmount, abi.TokenAmount, error) {
	bounds, err := c.full.StateDealProviderCollateralBounds(ctx, size, isVerified, types.EmptyTSK)
	if err != nil {
		return abi.TokenAmount{}, abi.TokenAmount{}, err
	}

	min := big.Mul(bounds.Min, big.NewInt(clientOverestimation.numerator))
	min = big.Div(min, big.NewInt(clientOverestimation.denominator))
	return min, bounds.Max, nil
}

// TODO: Remove dealID parameter, change publishCid to be cid.Cid (instead of pointer)
func (c *ClientNodeAdapter) OnDealSectorPreCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, proposal market2.DealProposal, publishCid *cid.Cid, cb storagemarket.DealSectorPreCommittedCallback) error {
	return c.scMgr.OnDealSectorPreCommitted(ctx, provider, marketactor.DealProposal(proposal), *publishCid, cb)
}

// TODO: Remove dealID parameter, change publishCid to be cid.Cid (instead of pointer)
func (c *ClientNodeAdapter) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, sectorNumber abi.SectorNumber, proposal market2.DealProposal, publishCid *cid.Cid, cb storagemarket.DealSectorCommittedCallback) error {
	return c.scMgr.OnDealSectorCommitted(ctx, provider, sectorNumber, marketactor.DealProposal(proposal), *publishCid, cb)
}

// TODO: Replace dealID parameter with DealProposal
func (c *ClientNodeAdapter) OnDealExpiredOrSlashed(ctx context.Context, dealID abi.DealID, onDealExpired storagemarket.DealExpiredCallback, onDealSlashed storagemarket.DealSlashedCallback) error {
	head, err := c.full.ChainHead(ctx)
	if err != nil {
		return xerrors.Errorf("client: failed to get chain head: %w", err)
	}

	sd, err := c.full.StateMarketStorageDeal(ctx, dealID, head.Key())
	if err != nil {
		return xerrors.Errorf("client: failed to look up deal %d on chain: %w", dealID, err)
	}

	// Called immediately to check if the deal has already expired or been slashed
	checkFunc := func(ctx context.Context, ts *types.TipSet) (done bool, more bool, err error) {
		if ts == nil {
			// keep listening for events
			return false, true, nil
		}

		// Check if the deal has already expired
		if sd.Proposal.EndEpoch <= ts.Height() {
			onDealExpired(nil)
			return true, false, nil
		}

		// If there is no deal assume it's already been slashed
		if sd.State.SectorStartEpoch < 0 {
			onDealSlashed(ts.Height(), nil)
			return true, false, nil
		}

		// No events have occurred yet, so return
		// done: false, more: true (keep listening for events)
		return false, true, nil
	}

	// Called when there was a match against the state change we're looking for
	// and the chain has advanced to the confidence height
	stateChanged := func(ts *types.TipSet, ts2 *types.TipSet, states events.StateChange, h abi.ChainEpoch) (more bool, err error) {
		// Check if the deal has already expired
		if ts2 == nil || sd.Proposal.EndEpoch <= ts2.Height() {
			onDealExpired(nil)
			return false, nil
		}

		// Timeout waiting for state change
		if states == nil {
			log.Error("timed out waiting for deal expiry")
			return false, nil
		}

		changedDeals, ok := states.(state.ChangedDeals)
		if !ok {
			panic("Expected state.ChangedDeals")
		}

		deal, ok := changedDeals[dealID]
		if !ok {
			// No change to deal
			return true, nil
		}

		// Deal was slashed
		if deal.To == nil {
			onDealSlashed(ts2.Height(), nil)
			return false, nil
		}

		return true, nil
	}

	// Called when there was a chain reorg and the state change was reverted
	revert := func(ctx context.Context, ts *types.TipSet) error {
		// TODO: Is it ok to just ignore this?
		log.Warn("deal state reverted; TODO: actually handle this!")
		return nil
	}

	// Watch for state changes to the deal
	match := c.dsMatcher.matcher(ctx, dealID)

	// Wait until after the end epoch for the deal and then timeout
	timeout := (sd.Proposal.EndEpoch - head.Height()) + 1
	if err := c.ev.StateChanged(checkFunc, stateChanged, revert, int(constants.MessageConfidence)+1, timeout, match); err != nil {
		return xerrors.Errorf("failed to set up state changed handler: %w", err)
	}

	return nil
}

func (c *ClientNodeAdapter) SignProposal(ctx context.Context, signer address.Address, proposal market2.DealProposal) (*market2.ClientDealProposal, error) {
	// TODO: output spec signed proposal
	buf, err := cborutil.Dump(&proposal)
	if err != nil {
		return nil, err
	}

	signer, err = c.full.StateAccountKey(ctx, signer, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	sig, err := c.singerService.WalletSign(ctx, signer, buf, wallet.MsgMeta{
		Type: wallet.MTDealProposal,
	})
	if err != nil {
		return nil, err
	}

	return &market2.ClientDealProposal{
		Proposal:        proposal,
		ClientSignature: *sig,
	}, nil
}

func (c *ClientNodeAdapter) GetDefaultWalletAddress(ctx context.Context) (address.Address, error) {
	return address.Address(c.cfg.DefaultMarketAddress), nil
}

func (c *ClientNodeAdapter) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	head, err := c.full.ChainHead(ctx)
	if err != nil {
		return nil, 0, err
	}

	return head.Key().Bytes(), head.Height(), nil
}

func (c *ClientNodeAdapter) WaitForMessage(ctx context.Context, mcid cid.Cid, cb func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error) error {
	receipt, err := c.full.StateWaitMsg(ctx, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
	if err != nil {
		return cb(0, nil, cid.Undef, err)
	}
	return cb(receipt.Receipt.ExitCode, receipt.Receipt.ReturnValue, receipt.Message, nil)
}

func (c *ClientNodeAdapter) GetMinerInfo(ctx context.Context, addr address.Address, encodedTs shared.TipSetToken) (*storagemarket.StorageProviderInfo, error) {
	tsk, err := types.TipSetKeyFromBytes(encodedTs)
	if err != nil {
		return nil, err
	}
	mi, err := c.full.StateMinerInfo(ctx, addr, tsk)
	if err != nil {
		return nil, err
	}

	out := utils.NewStorageProviderInfo(addr, mi.Worker, mi.SectorSize, *mi.PeerId, mi.Multiaddrs)
	return &out, nil
}

func (c *ClientNodeAdapter) SignBytes(ctx context.Context, signer address.Address, b []byte) (*crypto.Signature, error) {
	signer, err := c.full.StateAccountKey(ctx, signer, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	localSignature, err := c.singerService.WalletSign(ctx, signer, b, wallet.MsgMeta{
		Type: wallet.MTUnknown, // TODO: pass type here
	})
	if err != nil {
		return nil, err
	}
	return localSignature, nil
}

var _ storagemarket.StorageClientNode = &ClientNodeAdapter{}
