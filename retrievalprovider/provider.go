package retrievalprovider

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/dtutils"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/paychmgr"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

var queryTimeout = 5 * time.Minute

var log = logging.Logger("retrievaladapter")

type IRetrievalProvider interface {
	Stop() error
	Start(ctx context.Context) error
	ListDeals(ctx context.Context, params *types.RetrievalDealQueryParams) (map[retrievalmarket.ProviderDealIdentifier]*types.ProviderDealState, error)
}

var _ IRetrievalProvider = (*RetrievalProvider)(nil)

// RetrievalProvider is the production implementation of the RetrievalProvider interface
type RetrievalProvider struct {
	dataTransfer     network.ProviderDataTransfer
	network          rmnet.RetrievalMarketNetwork
	requestValidator *ProviderRequestValidator
	dagStore         stores.DAGStoreWrapper
	stores           *stores.ReadOnlyBlockstores

	retrievalDealRepo repo.IRetrievalDealRepo
	storageDealRepo   repo.StorageDealRepo

	retrievalStreamHandler *RetrievalStreamHandler

	transportListener *TransportsListener
}

// NewProvider returns a new retrieval Provider
func NewProvider(
	network rmnet.RetrievalMarketNetwork,
	dagStore stores.DAGStoreWrapper,
	dataTransfer network.ProviderDataTransfer,
	fullNode v1api.FullNode,
	payAPI *paychmgr.PaychAPI,
	repo repo.Repo,
	cfg *config.MarketConfig,
	rdf config.RetrievalDealFilter,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	gatewayMarketClient gateway.IMarketClient,
	transportLister *TransportsListener,
) (*RetrievalProvider, error) {
	storageDealsRepo := repo.StorageDealRepo()
	retrievalDealRepo := repo.RetrievalDealRepo()
	retrievalAskRepo := repo.RetrievalAskRepo()

	pieceInfo := &PieceInfo{dagStore, storageDealsRepo}
	p := &RetrievalProvider{
		dataTransfer:           dataTransfer,
		network:                network,
		dagStore:               dagStore,
		retrievalDealRepo:      retrievalDealRepo,
		storageDealRepo:        storageDealsRepo,
		stores:                 stores.NewReadOnlyBlockstores(),
		retrievalStreamHandler: NewRetrievalStreamHandler(cfg, retrievalAskRepo, retrievalDealRepo, storageDealsRepo, pieceInfo),
		transportListener:      transportLister,
	}

	retrievalHandler := NewRetrievalDealHandler(newProviderDealEnvironment(p, fullNode, payAPI), retrievalDealRepo, storageDealsRepo, gatewayMarketClient, pieceStorageMgr)
	p.requestValidator = NewProviderRequestValidator(cfg, storageDealsRepo, retrievalDealRepo, retrievalAskRepo, pieceInfo, rdf)
	transportConfigurer := dtutils.TransportConfigurer(network.ID(), &providerStoreGetter{retrievalDealRepo, p.stores})

	err := p.dataTransfer.RegisterVoucherType(retrievalmarket.DealProposalType, p.requestValidator)
	if err != nil {
		return nil, err
	}

	err = p.dataTransfer.RegisterVoucherType(retrievalmarket.DealPaymentType, p.requestValidator)
	if err != nil {
		return nil, err
	}

	err = p.dataTransfer.RegisterTransportConfigurer(retrievalmarket.DealProposalType, transportConfigurer)
	if err != nil {
		return nil, err
	}

	datatransferProcess := NewDataTransferHandler(retrievalHandler, retrievalDealRepo)
	dataTransfer.SubscribeToEvents(ProviderDataTransferSubscriber(datatransferProcess))
	return p, nil
}

// Stop stops handling incoming requests.
func (p *RetrievalProvider) Stop() error {
	p.transportListener.Stop()
	return p.network.StopHandlingRequests()
}

// Start begins listening for deals on the given host.
// Start must be called in order to accept incoming deals.
func (p *RetrievalProvider) Start(ctx context.Context) error {
	p.transportListener.Start()
	return p.network.SetDelegate(p.retrievalStreamHandler)
}

// ListDeals lists all known retrieval deals
func (p *RetrievalProvider) ListDeals(ctx context.Context, params *types.RetrievalDealQueryParams) (map[retrievalmarket.ProviderDealIdentifier]*types.ProviderDealState, error) {
	deals, err := p.retrievalDealRepo.ListDeals(ctx, params)
	if err != nil {
		return nil, err
	}

	dealMap := make(map[retrievalmarket.ProviderDealIdentifier]*types.ProviderDealState)
	for _, deal := range deals {
		dealMap[retrievalmarket.ProviderDealIdentifier{Receiver: deal.Receiver, DealID: deal.ID}] = deal
	}
	return dealMap, nil
}
