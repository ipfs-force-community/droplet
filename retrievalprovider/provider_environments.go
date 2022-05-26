package retrievalprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/dagstore"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-state-types/abi"
)

// CheckDealParams verifies the given deal params are acceptable
func CheckDealParams(ask *types.RetrievalAsk, pricePerByte abi.TokenAmount, paymentInterval uint64, paymentIntervalIncrease uint64, unsealPrice abi.TokenAmount) error {
	if pricePerByte.LessThan(ask.PricePerByte) {
		return errors.New("price per byte too low")
	}
	if paymentInterval > ask.PaymentInterval {
		return errors.New("payment interval too large")
	}
	if paymentIntervalIncrease > ask.PaymentIntervalIncrease {
		return errors.New("payment interval increase too large")
	}
	if !ask.UnsealPrice.Nil() && unsealPrice.LessThan(ask.UnsealPrice) {
		return errors.New("unseal price too small")
	}
	return nil
}

// ProviderDealEnvironment is a bridge to the environment a provider deal is executing in
// It provides access to relevant functionality on the retrieval provider
type ProviderDealEnvironment interface {
	PrepareBlockstore(ctx context.Context, dealID retrievalmarket.DealID, pieceCid cid.Cid) error
	DeleteStore(dealID retrievalmarket.DealID) error
	ResumeDataTransfer(context.Context, datatransfer.ChannelID) error
	CloseDataTransfer(context.Context, datatransfer.ChannelID) error
}

var _ ProviderDealEnvironment = new(providerDealEnvironment)

type providerDealEnvironment struct {
	p *RetrievalProvider
}

// PrepareBlockstore is called when the deal data has been unsealed and we need
// to add all blocks to a blockstore that is used to serve retrieval
func (pde *providerDealEnvironment) PrepareBlockstore(ctx context.Context, dealID retrievalmarket.DealID, pieceCid cid.Cid) error {
	// Load the blockstore that has the deal data
	//触发unseal过程
	bs, err := pde.p.dagStore.LoadShard(ctx, pieceCid)
	if err != nil {
		return fmt.Errorf("failed to load blockstore for piece %s: %w", pieceCid, err)
	}

	log.Debugf("adding blockstore for deal %d to tracker", dealID)
	_, err = pde.p.stores.Track(dealID.String(), bs)
	log.Debugf("added blockstore for deal %d to tracker", dealID)
	return err
}

func (pde *providerDealEnvironment) ResumeDataTransfer(ctx context.Context, chid datatransfer.ChannelID) error {
	return pde.p.dataTransfer.ResumeDataTransferChannel(ctx, chid)
}

func (pde *providerDealEnvironment) CloseDataTransfer(ctx context.Context, chid datatransfer.ChannelID) error {
	// When we close the data transfer, we also send a cancel message to the peer.
	// Make sure we don't wait too long to send the message.
	ctx, cancel := context.WithTimeout(ctx, shared.CloseDataTransferTimeout)
	defer cancel()

	err := pde.p.dataTransfer.CloseDataTransferChannel(ctx, chid)
	if shared.IsCtxDone(err) {
		log.Warnf("failed to send cancel data transfer channel %s to client within timeout %s",
			chid, shared.CloseDataTransferTimeout)
		return nil
	}
	return err
}

func (pde *providerDealEnvironment) DeleteStore(dealID retrievalmarket.DealID) error {
	// close the read-only blockstore and stop tracking it for the deal
	if err := pde.p.stores.Untrack(dealID.String()); err != nil {
		return fmt.Errorf("failed to clean read-only blockstore for deal %d: %w", dealID, err)
	}

	return nil
}

var _ dtutils.StoreGetter = &providerStoreGetter{}

type providerStoreGetter struct {
	deals  repo.IRetrievalDealRepo
	stores *stores.ReadOnlyBlockstores
}

func (psg *providerStoreGetter) Get(otherPeer peer.ID, dealID retrievalmarket.DealID) (bstore.Blockstore, error) {
	has, err := psg.deals.HasDeal(context.TODO(), otherPeer, dealID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deal state: %w", err)
	}

	if !has {
		return nil, fmt.Errorf("market has no deal for peer %s, deal %d", otherPeer, dealID)
	}

	//
	// When a request for data is received
	// 1. The data transfer layer calls Get to get the blockstore
	// 2. The data for the deal is unsealed
	// 3. The unsealed data is put into the blockstore (in this case a CAR file)
	// 4. The data is served from the blockstore (using blockstore.Get)
	//
	// So we use a "lazy" blockstore that can be returned in step 1
	// but is only accessed in step 4 after the data has been unsealed.
	//
	return newLazyBlockstore(func() (dagstore.ReadBlockstore, error) {
		return psg.stores.Get(dealID.String())
	}), nil
}
