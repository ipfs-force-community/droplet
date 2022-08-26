package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/filecoin-project/go-data-transfer/channelmonitor"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/ipfs-force-community/metrics"

	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	"github.com/filecoin-project/go-fil-markets/discovery"
	discoveryimpl "github.com/filecoin-project/go-fil-markets/discovery/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/imports"
	"github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/paychmgr"
	"github.com/filecoin-project/venus-market/v2/retrievalprovider"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
	marketevents "github.com/filecoin-project/venus-market/v2/utils"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/ipfs-force-community/venus-common-utils/journal"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type StorageProviderEvt struct {
	Event string
	Deal  storagemarket.MinerDeal
}

func NewLocalDiscovery(lc fx.Lifecycle, ds badger.ClientDealsDS) (*discoveryimpl.Local, error) {
	local, err := discoveryimpl.NewLocal(ds) //todo need new discoveryimpl base on sql
	if err != nil {
		return nil, err
	}
	local.OnReady(marketevents.ReadyLogger("discovery"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return local.Start(ctx)
		},
	})
	return local, nil
}

func RetrievalResolver(l *discoveryimpl.Local) discovery.PeerResolver {
	return discoveryimpl.Multi(l)
}

func NewClientImportMgr(ctx metrics.MetricsCtx, ns badger.ImportClientDS, r *config.HomeDir) (ClientImportMgr, error) {
	// store the imports under the repo's `imports` subdirectory.
	dir := filepath.Join(string(*r), "imports")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return imports.NewManager(ctx, ns, dir), nil
}

// NewClientGraphsyncDataTransfer returns a data transfer manager that just
// uses the clients's Client DAG service for transfers
func NewClientGraphsyncDataTransfer(lc fx.Lifecycle, h host.Host, gs network.Graphsync, dtDs badger.ClientTransferDS, homeDir *config.HomeDir) (network.ClientDataTransfer, error) {
	// go-data-transfer protocol retries:
	// 1s, 5s, 25s, 2m5s, 5m x 11 ~= 1 hour
	dtRetryParams := dtnet.RetryParameters(time.Second, 5*time.Minute, 15, 5)
	net := dtnet.NewFromLibp2pHost(h, dtRetryParams)

	transport := dtgstransport.NewTransport(h.ID(), gs)
	// data-transfer push / pull channel restart configuration:
	dtRestartConfig := dtimpl.ChannelRestartConfig(channelmonitor.Config{
		// Disable Accept and Complete timeouts until this issue is resolved:
		// https://github.com/filecoin-project/lotus/issues/6343#
		// Wait for the other side to respond to an Open channel message
		AcceptTimeout: 0,
		// Wait for the other side to send a Complete message once all
		// data has been sent / received
		CompleteTimeout: 0,

		// When an error occurs, wait a little while until all related errors
		// have fired before sending a restart message
		RestartDebounce: 10 * time.Second,
		// After sending a restart, wait for at least 1 minute before sending another
		RestartBackoff: time.Minute,
		// After trying to restart 3 times, give up and fail the transfer
		MaxConsecutiveRestarts: 3,
	})

	dt, err := dtimpl.NewDataTransfer(dtDs, net, transport, dtRestartConfig)
	if err != nil {
		return nil, err
	}

	dt.OnReady(marketevents.ReadyLogger("client data transfer"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			dt.SubscribeToEvents(marketevents.DataTransferLogger)
			return dt.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return dt.Stop(ctx)
		},
	})
	return dt, nil
}

// StorageBlockstoreAccessor returns the default storage blockstore accessor
// from the import manager.
func StorageBlockstoreAccessor(importmgr ClientImportMgr) storagemarket.BlockstoreAccessor {
	return storageprovider.NewImportsBlockstoreAccessor(importmgr)
}

// RetrievalBlockstoreAccessor returns the default retrieval blockstore accessor
// using the subdirectory `retrievals`
func RetrievalBlockstoreAccessor(r *config.HomeDir) (retrievalmarket.BlockstoreAccessor, error) {
	dir := filepath.Join(string(*r), "retrievals")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return retrievalprovider.NewCARBlockstoreAccessor(dir), nil
}

func StorageClient(lc fx.Lifecycle, h host.Host, dataTransfer network.ClientDataTransfer, discovery *discoveryimpl.Local,
	deals badger.ClientDatastore, scn storagemarket.StorageClientNode, accessor storagemarket.BlockstoreAccessor, j journal.Journal) (storagemarket.StorageClient, error) {
	// go-fil-markets protocol retries:
	// 1s, 5s, 25s, 2m5s, 5m x 11 ~= 1 hour
	marketsRetryParams := smnet.RetryParameters(time.Second, 5*time.Minute, 15, 5)
	net := smnet.NewFromLibp2pHost(h, marketsRetryParams)

	c, err := storageimpl.NewClient(net, dataTransfer, discovery, deals, scn, accessor, storageimpl.DealPollingInterval(time.Second*30))
	if err != nil {
		return nil, err
	}
	c.OnReady(marketevents.ReadyLogger("storage client"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			c.SubscribeToEvents(marketevents.StorageClientLogger)

			evtType := j.RegisterEventType("markets/storage/client", "state_change")
			c.SubscribeToEvents(StorageClientJournaler(j, evtType))

			return c.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return c.Stop()
		},
	})
	return c, nil
}

// RetrievalClient creates a new retrieval client attached to the client blockstore
func RetrievalClient(lc fx.Lifecycle, h host.Host, dt network.ClientDataTransfer, payAPI *paychmgr.PaychAPI, resolver discovery.PeerResolver,
	ds badger.RetrievalClientDS, fullApi v1api.FullNode, accessor retrievalmarket.BlockstoreAccessor, j journal.Journal) (retrievalmarket.RetrievalClient, error) {

	adapter := retrievalprovider.NewRetrievalClientNode(payAPI, fullApi)
	libP2pHost := rmnet.NewFromLibp2pHost(h)
	client, err := retrievalimpl.NewClient(libP2pHost, dt, adapter, resolver, ds, accessor)
	if err != nil {
		return nil, err
	}
	client.OnReady(marketevents.ReadyLogger("retrieval client"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			client.SubscribeToEvents(marketevents.RetrievalClientLogger)

			evtType := j.RegisterEventType("markets/retrieval/client", "state_change")
			client.SubscribeToEvents(RetrievalClientJournaler(j, evtType))

			return client.Start(ctx)
		},
	})
	return client, nil
}

type StorageClientEvt struct {
	Event string
	Deal  storagemarket.ClientDeal
}

type RetrievalClientEvt struct {
	Event string
	Deal  retrievalmarket.ClientDealState
}

// StorageClientJournaler records journal events from the piecestorage client.
func StorageClientJournaler(j journal.Journal, evtType journal.EventType) func(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
	return func(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
		j.RecordEvent(evtType, func() interface{} {
			return StorageClientEvt{
				Event: storagemarket.ClientEvents[event],
				Deal:  deal,
			}
		})
	}
}

// RetrievalClientJournaler records journal events from the retrieval client.
func RetrievalClientJournaler(j journal.Journal, evtType journal.EventType) func(event retrievalmarket.ClientEvent, deal retrievalmarket.ClientDealState) {
	return func(event retrievalmarket.ClientEvent, deal retrievalmarket.ClientDealState) {
		j.RecordEvent(evtType, func() interface{} {
			return RetrievalClientEvt{
				Event: retrievalmarket.ClientEvents[event],
				Deal:  deal,
			}
		})
	}
}

var MarketClientOpts = builder.Options(
	// Markets (common)
	builder.Override(new(*discoveryimpl.Local), NewLocalDiscovery),
	builder.Override(new(discovery.PeerResolver), RetrievalResolver),
	builder.Override(new(network.ClientDataTransfer), NewClientGraphsyncDataTransfer),

	builder.Override(new(ClientImportMgr), NewClientImportMgr),
	builder.Override(new(storagemarket.BlockstoreAccessor), StorageBlockstoreAccessor),

	builder.Override(new(retrievalmarket.BlockstoreAccessor), RetrievalBlockstoreAccessor),
	builder.Override(new(retrievalmarket.RetrievalClient), RetrievalClient),
	builder.Override(new(storagemarket.StorageClient), StorageClient),
)
