package retrievalprovider

import (
	"context"

	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"
)

type IAskHandler interface {
	GetAsk(address.Address) (*types.RetrievalAsk, error)
	SetAsk(*types.RetrievalAsk) error
	GetDynamicAsk(context.Context, address.Address, retrievalmarket.PricingInput, []abi.DealID) (retrievalmarket.Ask, error)
	GetAskForPayload(context.Context, address.Address, cid.Cid, []*types.MinerDeal, bool, peer.ID) (retrievalmarket.Ask, error)
}

var _ IAskHandler = (*AskHandler)(nil)

type AskHandler struct {
	storageRepo          repo.StorageDealRepo
	cidInfoRepo          repo.ICidInfoRepo
	askStore             repo.IRetrievalAskRepo
	node                 retrievalmarket.RetrievalProviderNode
	retrievalPricingFunc retrievalimpl.RetrievalPricingFunc
}

func NewAskHandler(r repo.Repo, node retrievalmarket.RetrievalProviderNode, retrievalPricingFunc retrievalimpl.RetrievalPricingFunc) IAskHandler {
	return &AskHandler{askStore: r.RetrievalAskRepo(), storageRepo: r.StorageDealRepo(), cidInfoRepo: r.CidInfoRepo(), node: node, retrievalPricingFunc: retrievalPricingFunc}
}

// GetAsk returns the current deal parameters this provider accepts
func (p *AskHandler) GetAsk(mAddr address.Address) (*types.RetrievalAsk, error) {
	return p.askStore.GetAsk(mAddr)
}

// GetAsk returns the current deal parameters this provider accepts
func (p *AskHandler) HasAsk(mAddr address.Address) (*types.RetrievalAsk, error) {
	return p.askStore.GetAsk(mAddr)
}

// SetAsk sets the deal parameters this provider accepts
func (p *AskHandler) SetAsk(ask *types.RetrievalAsk) error {
	return p.askStore.SetAsk(ask)
}

// GetDynamicAsk quotes a dynamic price for the retrieval deal by calling the user configured
// dynamic pricing function. It passes the static price parameters set in the Ask Store to the pricing function.
func (p *AskHandler) GetDynamicAsk(ctx context.Context, paymentAddr address.Address, input retrievalmarket.PricingInput, storageDeals []abi.DealID) (retrievalmarket.Ask, error) {
	dp, err := p.node.GetRetrievalPricingInput(ctx, input.PieceCID, storageDeals)
	if err != nil {
		return retrievalmarket.Ask{}, xerrors.Errorf("GetRetrievalPricingInput: %s", err)
	}

	// currAsk cannot be nil as we initialize the ask store with a default ask.
	// Users can then change the values in the ask store using SetAsk but not remove it.
	currAsk, err := p.GetAsk(paymentAddr) //todo use market payment address
	if err != nil || currAsk == nil {
		return retrievalmarket.Ask{}, xerrors.New("no ask configured in ask-store")
	}

	dp.PayloadCID = input.PayloadCID
	dp.PieceCID = input.PieceCID
	dp.Unsealed = input.Unsealed
	dp.Client = input.Client
	dp.CurrentAsk = retrievalmarket.Ask{
		PricePerByte:            currAsk.PricePerByte,
		UnsealPrice:             currAsk.UnsealPrice,
		PaymentInterval:         currAsk.PaymentInterval,
		PaymentIntervalIncrease: currAsk.PaymentIntervalIncrease,
	}

	ask, err := p.retrievalPricingFunc(ctx, dp)
	if err != nil {
		return retrievalmarket.Ask{}, xerrors.Errorf("retrievalPricingFunc: %w", err)
	}
	return ask, nil
}

func (p *AskHandler) GetAskForPayload(ctx context.Context, paymentAddr address.Address, payloadCid cid.Cid, minerDeals []*types.MinerDeal, isUnsealed bool, client peer.ID) (retrievalmarket.Ask, error) {
	if len(minerDeals) == 0 {
		return retrievalmarket.Ask{}, xerrors.Errorf("get ask for payload, miner deals not exit")
	}
	var deals []abi.DealID
	for _, minerDeal := range minerDeals {
		deals = append(deals, minerDeal.DealID)
	}

	input := retrievalmarket.PricingInput{
		// piece from which the payload will be retrieved
		PieceCID: minerDeals[0].Proposal.PieceCID,

		PayloadCID: payloadCid,
		Unsealed:   isUnsealed,
		Client:     client,
	}

	return p.GetDynamicAsk(ctx, paymentAddr, input, deals)
}
