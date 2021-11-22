package retrievalprovider

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/types"
	"math"
	"time"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/migrations"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/network"
)

var queryTimeout = 5 * time.Second

type IRetrievalProvider interface {
	Stop() error
	Start(ctx context.Context) error
	ListDeals() (map[retrievalmarket.ProviderDealIdentifier]*types.ProviderDealState, error)
}

// RetrievalProvider is the production implementation of the RetrievalProvider interface
type RetrievalProvider struct {
	dataTransfer     network.ProviderDataTransfer
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

// NewProvider returns a new retrieval Provider
func NewProvider(node retrievalmarket.RetrievalProviderNode,
	network rmnet.RetrievalMarketNetwork,
	askHandler IAskHandler,
	dagStore stores.DAGStoreWrapper,
	dataTransfer network.ProviderDataTransfer,
	repo repo.Repo,
	cfg *config.MarketConfig,
) (*RetrievalProvider, error) {
	storageDealsRepo := repo.StorageDealRepo()
	retrievalDealRepo := repo.RetrievalDealRepo()

	pieceInfo := &PieceInfo{cidInfoRepo: repo.CidInfoRepo(), dealRepo: repo.StorageDealRepo()}
	p := &RetrievalProvider{
		dataTransfer:           dataTransfer,
		node:                   node,
		network:                network,
		dagStore:               dagStore,
		askHandler:             askHandler,
		retrievalDealRepo:      repo.RetrievalDealRepo(),
		storageDealRepo:        repo.StorageDealRepo(),
		stores:                 stores.NewReadOnlyBlockstores(),
		retrievalStreamHandler: NewRetrievalStreamHandler(askHandler, retrievalDealRepo, storageDealsRepo, pieceInfo, address.Address(cfg.RetrievalPaymentAddress)),
	}
	retrievalHandler := NewRetrievalDealHandler(&providerDealEnvironment{p}, retrievalDealRepo)
	p.requestValidator = NewProviderRequestValidator(address.Address(cfg.RetrievalPaymentAddress), storageDealsRepo, retrievalDealRepo, pieceInfo, askHandler)
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
func (p *RetrievalProvider) Stop() error {
	return p.network.StopHandlingRequests()
}

// Start begins listening for deals on the given host.
// Start must be called in order to accept incoming deals.
func (p *RetrievalProvider) Start(ctx context.Context) error {
	return p.network.SetDelegate(p.retrievalStreamHandler)
}

// ListDeals lists all known retrieval deals
func (p *RetrievalProvider) ListDeals() (map[retrievalmarket.ProviderDealIdentifier]*types.ProviderDealState, error) {
	deals, err := p.retrievalDealRepo.ListDeals(0, math.MaxInt32)
	if err != nil {
		return nil, err
	}

	dealMap := make(map[retrievalmarket.ProviderDealIdentifier]*types.ProviderDealState)
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
