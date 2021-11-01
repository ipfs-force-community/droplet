package retrievaladapter

import (
	"context"
	"errors"
	"math"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/migrations"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/hannahhoward/go-pubsub"
	"golang.org/x/xerrors"
)

var queryTimeout = 5 * time.Second

type IRetrievalProvider interface {
	Stop() error
	Start(ctx context.Context) error
	ListDeals() (map[retrievalmarket.ProviderDealIdentifier]*retrievalmarket.ProviderDealState, error)
}

// RetrievalProviderV2 is the production implementation of the RetrievalProvider interface
type RetrievalProviderV2 struct {
	dataTransfer     datatransfer.Manager
	node             retrievalmarket.RetrievalProviderNode
	network          rmnet.RetrievalMarketNetwork
	requestValidator *ProviderRequestValidator
	reValidator      *ProviderRevalidator
	disableNewDeals  bool
	dagStore         stores.DAGStoreWrapper
	stores           *stores.ReadOnlyBlockstores

	retrievalDealRepo repo.IRetrievalDealRepo
	storageDealRepo   repo.StorageDealRepo

	askHandler             IAskHandler
	retrievalStreamHandler *RetrievalStreamHandler
}

type internalProviderEvent struct {
	evt   retrievalmarket.ProviderEvent
	state retrievalmarket.ProviderDealState
}

func providerDispatcher(evt pubsub.Event, subscriberFn pubsub.SubscriberFn) error {
	ie, ok := evt.(internalProviderEvent)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb, ok := subscriberFn.(retrievalmarket.ProviderSubscriber)
	if !ok {
		return errors.New("wrong type of event")
	}
	log.Debugw("process retrieval provider] listeners", "name", retrievalmarket.ProviderEvents[ie.evt], "proposal cid", ie.state.ID)
	cb(ie.evt, ie.state)
	return nil
}

// NewProvider returns a new retrieval Provider
func NewProvider(node retrievalmarket.RetrievalProviderNode,
	network rmnet.RetrievalMarketNetwork,
	dagStore stores.DAGStoreWrapper,
	dataTransfer datatransfer.Manager,
	retrievalPricingFunc retrievalimpl.RetrievalPricingFunc,
	repo repo.Repo,
) (*RetrievalProviderV2, error) {
	if retrievalPricingFunc == nil {
		return nil, xerrors.New("retrievalPricingFunc is nil")
	}

	askHandler := NewAskHandler(repo, node, retrievalPricingFunc)

	storageDealsRepo := repo.StorageDealRepo()
	retrievalDealRepo := repo.RetrievalDealRepo()

	pieceInfo := &PieceInfo{cidInfoRepo: repo.CidInfoRepo(), dealRepo: repo.StorageDealRepo()}
	p := &RetrievalProviderV2{
		dataTransfer:           dataTransfer,
		node:                   node,
		network:                network,
		dagStore:               dagStore,
		askHandler:             askHandler,
		stores:                 stores.NewReadOnlyBlockstores(),
		retrievalStreamHandler: NewRetrievalStreamHandler(askHandler, retrievalDealRepo, storageDealsRepo, pieceInfo),
	}
	retrievalHandler := NewRetrievalDealHandler(&providerDealEnvironment{p}, retrievalDealRepo)
	p.requestValidator = NewProviderRequestValidator(storageDealsRepo, retrievalDealRepo, pieceInfo, askHandler)
	transportConfigurer := dtutils.TransportConfigurer(network.ID(), &providerStoreGetter{retrievalDealRepo, p.stores})
	p.reValidator = NewProviderRevalidator(p.node, retrievalDealRepo, retrievalHandler)

	var err error
	if p.disableNewDeals {
		err = p.dataTransfer.RegisterVoucherType(&migrations.DealProposal0{}, p.requestValidator)
		if err != nil {
			return nil, err
		}
		err = p.dataTransfer.RegisterRevalidator(&migrations.DealPayment0{}, p.reValidator)
		if err != nil {
			return nil, err
		}
	} else {
		err = p.dataTransfer.RegisterVoucherType(&retrievalmarket.DealProposal{}, p.requestValidator)
		if err != nil {
			return nil, err
		}
		err = p.dataTransfer.RegisterVoucherType(&migrations.DealProposal0{}, p.requestValidator)
		if err != nil {
			return nil, err
		}

		err = p.dataTransfer.RegisterRevalidator(&retrievalmarket.DealPayment{}, p.reValidator)
		if err != nil {
			return nil, err
		}
		err = p.dataTransfer.RegisterRevalidator(&migrations.DealPayment0{}, NewLegacyRevalidator(p.reValidator))
		if err != nil {
			return nil, err
		}

		err = p.dataTransfer.RegisterVoucherResultType(&retrievalmarket.DealResponse{})
		if err != nil {
			return nil, err
		}

		err = p.dataTransfer.RegisterTransportConfigurer(&retrievalmarket.DealProposal{}, transportConfigurer)
		if err != nil {
			return nil, err
		}
	}
	err = p.dataTransfer.RegisterVoucherResultType(&migrations.DealResponse0{})
	if err != nil {
		return nil, err
	}
	err = p.dataTransfer.RegisterTransportConfigurer(&migrations.DealProposal0{}, transportConfigurer)
	if err != nil {
		return nil, err
	}
	datatransferProcess := NewDataTransferHandler(retrievalHandler, retrievalDealRepo)
	dataTransfer.SubscribeToEvents(ProviderDataTransferSubscriber(datatransferProcess))
	return p, nil
}

// Stop stops handling incoming requests.
func (p *RetrievalProviderV2) Stop() error {
	return p.network.StopHandlingRequests()
}

// Start begins listening for deals on the given host.
// Start must be called in order to accept incoming deals.
func (p *RetrievalProviderV2) Start(ctx context.Context) error {
	return p.network.SetDelegate(p.retrievalStreamHandler)
}

// ListDeals lists all known retrieval deals
func (p *RetrievalProviderV2) ListDeals() (map[retrievalmarket.ProviderDealIdentifier]*retrievalmarket.ProviderDealState, error) {
	deals, err := p.retrievalDealRepo.ListDeals(0, math.MaxInt32)
	if err != nil {
		return nil, err
	}

	dealMap := make(map[retrievalmarket.ProviderDealIdentifier]*retrievalmarket.ProviderDealState)
	for _, deal := range deals {
		dealMap[retrievalmarket.ProviderDealIdentifier{Receiver: deal.Receiver, DealID: deal.ID}] = deal
	}
	return dealMap, nil
}

// DefaultPricingFunc is the default pricing policy that will be used to price retrieval deals.
var DefaultPricingFunc = func(VerifiedDealsFreeTransfer bool) func(ctx context.Context, pricingInput retrievalmarket.PricingInput) (retrievalmarket.Ask, error) {
	return func(ctx context.Context, pricingInput retrievalmarket.PricingInput) (retrievalmarket.Ask, error) {
		ask := pricingInput.CurrentAsk

		// don't charge for Unsealing if we have an Unsealed copy.
		if pricingInput.Unsealed {
			ask.UnsealPrice = big.Zero()
		}

		// don't charge for data transfer for verified deals if it's been configured to do so.
		if pricingInput.VerifiedDeal && VerifiedDealsFreeTransfer {
			ask.PricePerByte = big.Zero()
		}

		return ask, nil
	}
}
