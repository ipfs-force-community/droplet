package storageprovider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"

	"github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/config"
	types2 "github.com/ipfs-force-community/droplet/v2/types"

	"github.com/filecoin-project/venus/venus-shared/actors"
	marketactor "github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	marketTypes "github.com/filecoin-project/venus/venus-shared/types/market"
)

type dealPublisherAPI interface {
	ChainHead(context.Context) (*types.TipSet, error)
	StateMinerInfo(context.Context, address.Address, types.TipSetKey) (types.MinerInfo, error)

	WalletBalance(context.Context, address.Address) (types.BigInt, error)
	WalletHas(context.Context, address.Address) (bool, error)
	StateAccountKey(context.Context, address.Address, types.TipSetKey) (address.Address, error)
	StateLookupID(context.Context, address.Address, types.TipSetKey) (address.Address, error)

	PushMessage(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec) (cid.Cid, error)
}

type DealPublisher struct {
	api dealPublisherAPI

	cfg *config.MarketConfig

	lk         sync.Mutex
	publishers map[address.Address]*singleDealPublisher
}

func NewDealPublisherWrapper(
	cfg *config.MarketConfig,
) func(lc fx.Lifecycle, full v1api.FullNode, msgClient clients.IMixMessage) *DealPublisher {
	return func(lc fx.Lifecycle, full v1api.FullNode, msgClient clients.IMixMessage) *DealPublisher {
		dp := &DealPublisher{
			api: struct {
				v1api.FullNode
				clients.IMixMessage
			}{full, msgClient},
			cfg:        cfg,
			publishers: map[address.Address]*singleDealPublisher{},
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				dp.lk.Lock()
				for _, p := range dp.publishers {
					p.Shutdown()
				}
				dp.lk.Unlock()
				return nil
			},
		})
		return dp
	}
}

// PendingDeals returns the list of deals that are queued up to be published
func (p *DealPublisher) PendingDeals() map[address.Address]marketTypes.PendingDealInfo {
	p.lk.Lock()
	defer p.lk.Unlock()

	ret := make(map[address.Address]marketTypes.PendingDealInfo)

	// Filter out deals whose context has been cancelled
	for addr, publisher := range p.publishers {
		pdi := publisher.pendingDeals()
		if len(pdi.Deals) > 0 {
			ret[addr] = pdi
		}
	}
	return ret
}

// ForcePublishPendingDeals publishes all pending deals without waiting for
// the publish period to elapse
func (p *DealPublisher) ForcePublishPendingDeals() {
	p.lk.Lock()
	defer p.lk.Unlock()

	log.Infof("force publishing deals")
	for _, singlePublisher := range p.publishers {
		singlePublisher.forcePublishPendingDeals()
	}
}

func (p *DealPublisher) Publish(ctx context.Context, deal types.ClientDealProposal) (cid.Cid, error) {
	pdeal := newPendingDeal(ctx, deal)

	p.lk.Lock()
	providerAddr := deal.Proposal.Provider
	publisher, ok := p.publishers[providerAddr]
	if !ok {
		pCfg, err := p.cfg.MinerProviderConfig(providerAddr, true)
		if err != nil {
			p.lk.Unlock()
			return cid.Undef, err
		}
		addrs := config.CfgAddrArrToNative(pCfg.DealPublishAddress)

		publisher = newDealPublisher(
			p.api,
			addrs,
			pCfg.MaxDealsPerPublishMsg,
			time.Duration(pCfg.PublishMsgPeriod),
			&types.MessageSendSpec{MaxFee: abi.TokenAmount(pCfg.MaxPublishDealsFee)})
		p.publishers[providerAddr] = publisher
	}
	publisher.processNewDeal(pdeal)
	p.lk.Unlock()
	// Wait for the deal to be submitted
	select {
	case <-ctx.Done():
		return cid.Undef, ctx.Err()
	case res := <-pdeal.Result:
		return res.msgCid, res.err
	}
}

// singleDealPublisher batches deal publishing so that many deals can be included in
// a single publish message. This saves gas for miners that publish deals
// frequently.
// When a deal is submitted, the singleDealPublisher waits a configurable amount of
// time for other deals to be submitted before sending the publish message.
// There is a configurable maximum number of deals that can be included in one
// message. When the limit is reached the singleDealPublisher immediately submits a
// publish message with all deals in the queue.
type singleDealPublisher struct {
	api          dealPublisherAPI
	publishAddrs []address.Address

	ctx      context.Context
	Shutdown context.CancelFunc

	maxDealsPerPublishMsg  uint64
	publishPeriod          time.Duration
	publishSpec            *types.MessageSendSpec
	cancelWaitForMoreDeals context.CancelFunc
	publishPeriodStart     time.Time

	lk      sync.Mutex
	pending []*pendingDeal
}

// A deal that is queued to be published
type pendingDeal struct {
	ctx    context.Context
	deal   types.ClientDealProposal
	Result chan publishResult
}

// The result of publishing a deal
type publishResult struct {
	msgCid cid.Cid
	err    error
}

func newPendingDeal(ctx context.Context, deal types.ClientDealProposal) *pendingDeal {
	return &pendingDeal{
		ctx:    ctx,
		deal:   deal,
		Result: make(chan publishResult),
	}
}

func newDealPublisher(
	dpapi dealPublisherAPI,
	publishAddrs []address.Address,
	maxDealsPerPublishMsg uint64,
	publishPeriod time.Duration,
	publishSpec *types.MessageSendSpec,
) *singleDealPublisher {
	ctx, cancel := context.WithCancel(context.Background())
	return &singleDealPublisher{
		api:                   dpapi,
		publishAddrs:          publishAddrs,
		ctx:                   ctx,
		Shutdown:              cancel,
		maxDealsPerPublishMsg: maxDealsPerPublishMsg,
		publishPeriod:         publishPeriod,
		publishSpec:           publishSpec,
	}
}

// PendingDeals returns the list of deals that are queued up to be published
func (p *singleDealPublisher) pendingDeals() marketTypes.PendingDealInfo {
	p.lk.Lock()
	defer p.lk.Unlock()

	// Filter out deals whose context has been cancelled
	deals := make([]*pendingDeal, 0, len(p.pending))
	for _, dl := range p.pending {
		if dl.ctx.Err() == nil {
			deals = append(deals, dl)
		}
	}

	pending := make([]types.ClientDealProposal, len(deals))
	for i, deal := range deals {
		pending[i] = deal.deal
	}

	return marketTypes.PendingDealInfo{
		Deals:              pending,
		PublishPeriodStart: p.publishPeriodStart,
		PublishPeriod:      p.publishPeriod,
	}
}

// ForcePublishPendingDeals publishes all pending deals without waiting for
// the publish period to elapse
func (p *singleDealPublisher) forcePublishPendingDeals() {
	p.lk.Lock()
	defer p.lk.Unlock()

	log.Infof("force publishing deals")
	p.publishAllDeals()
}

func (p *singleDealPublisher) processNewDeal(pdeal *pendingDeal) {
	p.lk.Lock()
	defer p.lk.Unlock()

	// Filter out any cancelled deals
	p.filterCancelledDeals()

	// If all deals have been cancelled, clear the wait-for-deals timer
	if len(p.pending) == 0 && p.cancelWaitForMoreDeals != nil {
		p.cancelWaitForMoreDeals()
		p.cancelWaitForMoreDeals = nil
	}

	// Make sure the new deal hasn't been cancelled
	if pdeal.ctx.Err() != nil {
		return
	}

	pDealPropCid, err := pdeal.deal.Proposal.Cid()
	if err != nil {
		log.Warn("failed to calculate proposal CID for new pending Deal with piece cid %s", pdeal.deal.Proposal.PieceCID)
		return
	}

	// Sanity check that new deal isn't already in the queue
	for _, pd := range p.pending {
		pdPropCid, err := pd.deal.Proposal.Cid()
		if err != nil {
			log.Warn("failed to calculate proposal CID for pending Deal already in publish queue with piece cid %s", pd.deal.Proposal.PieceCID)
			return
		}

		if pdPropCid.Equals(pDealPropCid) {
			log.Warn("tried to process new pending deal with piece CID %s that is already in publish queue; returning", pdeal.deal.Proposal.PieceCID)
			return
		}
	}

	// Add the new deal to the queue
	p.pending = append(p.pending, pdeal)
	log.Infof("add deal with piece CID %s to publish deals queue - %d deals in queue (max queue size %d)",
		pdeal.deal.Proposal.PieceCID, len(p.pending), p.maxDealsPerPublishMsg)

	// If the maximum number of deals per message has been reached or we're not batching, send a
	// publish message
	if uint64(len(p.pending)) >= p.maxDealsPerPublishMsg || p.publishPeriod == 0 {
		log.Infof("publish deals queue has reached max size of %d, publishing deals", p.maxDealsPerPublishMsg)
		p.publishAllDeals()
		return
	}

	// Otherwise wait for more deals to arrive or the timeout to be reached
	p.waitForMoreDeals()
}

func (p *singleDealPublisher) waitForMoreDeals() {
	// Check if we're already waiting for deals
	if !p.publishPeriodStart.IsZero() {
		elapsed := types2.Clock.Since(p.publishPeriodStart)
		log.Infof("%s elapsed of / %s until publish deals queue is published",
			elapsed, p.publishPeriod)
		return
	}

	// Set a timeout to wait for more deals to arrive
	log.Infof("waiting publish deals queue period of %s before publishing", p.publishPeriod)
	ctx, cancel := context.WithCancel(p.ctx)
	p.publishPeriodStart = types2.Clock.Now()
	p.cancelWaitForMoreDeals = cancel

	go func() {
		timer := types2.Clock.NewTimer(p.publishPeriod)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.Chan():
			p.lk.Lock()
			defer p.lk.Unlock()

			// The timeout has expired so publish all pending deals
			log.Infof("publish deals queue period of %s has expired, publishing deals", p.publishPeriod)
			p.publishAllDeals()
		}
	}()
}

func (p *singleDealPublisher) publishAllDeals() {
	// If the timeout hasn't yet been cancelled, cancel it
	if p.cancelWaitForMoreDeals != nil {
		p.cancelWaitForMoreDeals()
		p.cancelWaitForMoreDeals = nil
		p.publishPeriodStart = time.Time{}
	}

	// Filter out any deals that have been cancelled
	p.filterCancelledDeals()
	deals := p.pending[:]
	p.pending = nil

	// Send the publish message
	go p.publishReady(deals)
}

func (p *singleDealPublisher) publishReady(ready []*pendingDeal) {
	if len(ready) == 0 {
		return
	}

	// onComplete is called when the publish message has been sent or there
	// was an error
	onComplete := func(pd *pendingDeal, msgCid cid.Cid, err error) {
		// Send the publish result on the pending deal's Result channel
		res := publishResult{
			msgCid: msgCid,
			err:    err,
		}
		select {
		case <-p.ctx.Done():
		case <-pd.ctx.Done():
		case pd.Result <- res:
		}
	}

	// Validate each deal to make sure it can be published
	validated := make([]*pendingDeal, 0, len(ready))
	deals := make([]types.ClientDealProposal, 0, len(ready))
	for _, pd := range ready {
		// Validate the deal
		if err := p.validateDeal(pd.deal); err != nil {
			// Validation failed, complete immediately with an error
			go onComplete(pd, cid.Undef, err)
			continue
		}

		validated = append(validated, pd)
		deals = append(deals, pd.deal)
	}

	// Send the publish message
	msgCid, err := p.publishDealProposals(deals)

	// Signal that each deal has been published
	for _, pd := range validated {
		go onComplete(pd, msgCid, err)
	}
}

// validateDeal checks that the deal proposal start epoch hasn't already
// elapsed
func (p *singleDealPublisher) validateDeal(deal types.ClientDealProposal) error {
	head, err := p.api.ChainHead(p.ctx)
	if err != nil {
		return err
	}
	if head.Height() > deal.Proposal.StartEpoch {
		return fmt.Errorf(
			"cannot publish deal with piece CID %s: current epoch %d has passed deal proposal start epoch %d",
			deal.Proposal.PieceCID, head.Height(), deal.Proposal.StartEpoch)
	}
	return nil
}

// Sends the publish message
func (p *singleDealPublisher) publishDealProposals(deals []types.ClientDealProposal) (cid.Cid, error) {
	if len(deals) == 0 {
		return cid.Undef, nil
	}

	log.Infof("publishing %d deals in publish deals queue with piece CIDs: %s", len(deals), pieceCids(deals))

	provider := deals[0].Proposal.Provider
	for _, dl := range deals {
		if dl.Proposal.Provider != provider {
			msg := fmt.Sprintf("publishing %d deals failed: ", len(deals)) +
				"not all deals are for same provider: " +
				fmt.Sprintf("deal with piece CID %s is for provider %s ", deals[0].Proposal.PieceCID, deals[0].Proposal.Provider) +
				fmt.Sprintf("but deal with piece CID %s is for provider %s", dl.Proposal.PieceCID, dl.Proposal.Provider)
			return cid.Undef, fmt.Errorf(msg)
		}
	}

	mi, err := p.api.StateMinerInfo(p.ctx, provider, types.EmptyTSK)
	if err != nil {
		return cid.Undef, err
	}

	params, err := actors.SerializeParams(&types.PublishStorageDealsParams{
		Deals: deals,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("serializing PublishStorageDeals params failed: %w", err)
	}

	addr, _, err := pickAddress(p.ctx, p.api, mi, big.Zero(), big.Zero(), p.publishAddrs)
	if err != nil {
		return cid.Undef, fmt.Errorf("selecting address for publishing deals: %w", err)
	}

	msgId, err := p.api.PushMessage(
		p.ctx,
		&types.Message{
			To:     marketactor.Address,
			From:   addr,
			Value:  types.NewInt(0),
			Method: builtin.MethodsMarket.PublishStorageDeals,
			Params: params,
		}, p.publishSpec)

	if err != nil {
		return cid.Undef, err
	}
	return msgId, nil
}

func pieceCids(deals []types.ClientDealProposal) string {
	cids := make([]string, 0, len(deals))
	for _, dl := range deals {
		cids = append(cids, dl.Proposal.PieceCID.String())
	}
	return strings.Join(cids, ", ")
}

// filter out deals that have been cancelled
func (p *singleDealPublisher) filterCancelledDeals() {
	i := 0
	for _, pd := range p.pending {
		if pd.ctx.Err() == nil {
			p.pending[i] = pd
			i++
		}
	}
	p.pending = p.pending[:i]
}

func pickAddress(ctx context.Context, a dealPublisherAPI, mi types.MinerInfo, goodFunds, minFunds abi.TokenAmount, addrs []address.Address) (address.Address, abi.TokenAmount, error) {
	leastBad := mi.Worker //default to worker
	bestAvail := minFunds

	ctl := map[address.Address]struct{}{}
	for _, a := range append(mi.ControlAddresses, mi.Owner, mi.Worker) {
		ctl[a] = struct{}{}
	}

	for _, addr := range addrs {
		if addr.Protocol() != address.ID {
			var err error
			addr, err = a.StateLookupID(ctx, addr, types.EmptyTSK)
			if err != nil {
				log.Warnw("looking up control address", "address", addr, "error", err)
				continue
			}
		}

		if _, ok := ctl[addr]; !ok {
			log.Warnw("non-control address configured for sending messages", "address", addr)
			continue
		}

		if maybeUseAddress(ctx, a, addr, goodFunds, &leastBad, &bestAvail) {
			return leastBad, bestAvail, nil
		}
	}

	log.Warnw("No address had enough funds to for full message Fee, selecting least bad address", "address", leastBad, "balance", types.FIL(bestAvail), "optimalFunds", types.FIL(goodFunds), "minFunds", types.FIL(minFunds))

	return leastBad, bestAvail, nil
}

func maybeUseAddress(ctx context.Context, a dealPublisherAPI, addr address.Address, goodFunds abi.TokenAmount, leastBad *address.Address, bestAvail *abi.TokenAmount) bool {
	b, err := a.WalletBalance(ctx, addr)
	if err != nil {
		log.Errorw("checking control address balance", "addr", addr, "error", err)
		return false
	}

	if b.GreaterThanEqual(goodFunds) {
		k, err := a.StateAccountKey(ctx, addr, types.EmptyTSK)
		if err != nil {
			log.Errorw("getting account key", "error", err)
			return false
		}

		have, err := a.WalletHas(ctx, k)
		if err != nil {
			log.Errorw("failed to check control address", "addr", addr, "error", err)
			return false
		}

		if !have {
			log.Errorw("don't have key", "key", k, "address", addr)
			return false
		}

		*leastBad = addr
		*bestAvail = b
		return true
	}

	if b.GreaterThan(*bestAvail) {
		*leastBad = addr
		*bestAvail = b
	}

	log.Warnw("address didn't have enough funds to send message", "address", addr, "required", types.FIL(goodFunds), "balance", types.FIL(b))
	return false
}
