package storageprovider

import (
	"context"
	"fmt"
	"github.com/filecoin-project/venus-market/utils"
	"io"
	"time"

	"github.com/hannahhoward/go-pubsub"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/filecoin-project/venus-market/types"
)

// StorageProviderV2 provides an interface to the storage market for a single
// storage miner.
type StorageProviderV2 interface {

	// Start initializes deal processing on a StorageProvider and restarts in progress deals.
	// It also registers the provider with a StorageMarketNetwork so it can receive incoming
	// messages on the storage market's libp2p protocols
	Start(ctx context.Context) error

	// Stop terminates processing of deals on a StorageProvider
	Stop() error

	// SetAsk configures the storage miner's ask with the provided prices (for unverified and verified deals),
	// duration, and options. Any previously-existing ask is replaced.
	SetAsk(mAddr address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error

	// GetAsk returns the storage miner's ask, or nil if one does not exist.
	GetAsk(mAddr address.Address) (*storagemarket.SignedStorageAsk, error)

	// ListLocalDeals lists deals processed by this storage provider
	ListLocalDeals(mAddr address.Address) ([]storagemarket.MinerDeal, error)

	// AddStorageCollateral adds storage collateral
	AddStorageCollateral(ctx context.Context, mAddr address.Address, amount abi.TokenAmount) error

	// GetStorageCollateral returns the current collateral balance
	GetStorageCollateral(ctx context.Context, mAddr address.Address) (storagemarket.Balance, error)

	// ImportDataForDeal manually imports data for an offline storage deal
	ImportDataForDeal(ctx context.Context, propCid cid.Cid, data io.Reader) error

	// SubscribeToEvents listens for events that happen related to storage deals on a provider
	SubscribeToEvents(subscriber storagemarket.ProviderSubscriber) shared.Unsubscribe
}

type StorageProviderV2Impl struct {
	net smnet.StorageMarketNetwork

	spn       StorageProviderNode
	fs        filestore.FileStore
	conns     *connmanager.ConnManager
	storedAsk IStorageAsk

	pubSub *pubsub.PubSub

	unsubDataTransfer datatransfer.Unsubscribe

	dealStore       repo.StorageDealRepo
	dealProcess     StorageDealProcess
	transferProcess TransferProcess
	storageReceiver smnet.StorageReceiver
	minerMgr        minermgr.IMinerMgr
}

type internalProviderEvent struct {
	evt  storagemarket.ProviderEvent
	deal storagemarket.MinerDeal
}

func providerDispatcher(evt pubsub.Event, fn pubsub.SubscriberFn) error {
	ie, ok := evt.(internalProviderEvent)
	if !ok {
		return xerrors.New("wrong type of event")
	}
	cb, ok := fn.(storagemarket.ProviderSubscriber)
	if !ok {
		return xerrors.New("wrong type of callback")
	}
	cb(ie.evt, ie.deal)
	return nil
}

// NewStorageProviderV2 returns a new storage provider
func NewStorageProviderV2(
	storedAsk IStorageAsk,
	h host.Host,
	homeDir *config.HomeDir,
	pieceStorage piecestorage.IPieceStorage,
	dataTransfer network.ProviderDataTransfer,
	spn StorageProviderNode,
	dagStore stores.DAGStoreWrapper,
	repo repo.Repo,
	minerMgr minermgr.IMinerMgr,
) (StorageProviderV2, error) {
	net := smnet.NewFromLibp2pHost(h)

	store, err := filestore.NewLocalFileStore(filestore.OsPath(string(*homeDir)))
	if err != nil {
		return nil, err
	}

	spV2 := &StorageProviderV2Impl{
		net: net,

		spn:       spn,
		fs:        store,
		conns:     connmanager.NewConnManager(),
		storedAsk: storedAsk,

		pubSub: pubsub.New(providerDispatcher),

		dealStore: repo.StorageDealRepo(),

		minerMgr: minerMgr,
	}

	dealProcess, err := NewStorageDealProcessImpl(spV2.conns, newPeerTagger(spV2.net), spV2.spn, spV2.dealStore, spV2.storedAsk, spV2.fs, minerMgr, repo, pieceStorage, dataTransfer, dagStore)
	if err != nil {
		return nil, err
	}
	spV2.dealProcess = dealProcess

	spV2.transferProcess = NewDataTransferProcess(dealProcess, spV2.dealStore)
	// register a data transfer event handler -- this will send events to the state machines based on DT events
	spV2.unsubDataTransfer = dataTransfer.SubscribeToEvents(ProviderDataTransferSubscriber(spV2.transferProcess)) // fsm.Group

	storageReceiver, err := NewStorageDealStream(spV2.conns, spV2.storedAsk, spV2.spn, spV2.dealStore, spV2.net, spV2.fs, dealProcess)
	if err != nil {
		return nil, err
	}
	spV2.storageReceiver = storageReceiver

	return spV2, nil
}

// Start initializes deal processing on a StorageProvider and restarts in progress deals.
// It also registers the provider with a StorageMarketNetwork so it can receive incoming
// messages on the storage market's libp2p protocols
func (p *StorageProviderV2Impl) Start(ctx context.Context) error {
	err := p.net.SetDelegate(p.storageReceiver)
	if err != nil {
		return err
	}

	go func() {
		err := p.start(ctx)
		if err != nil {
			log.Error(err.Error())
		}
	}()

	return nil
}

func (p *StorageProviderV2Impl) start(ctx context.Context) error {
	// Run datastore and DAG store migrations
	addrs, err := p.minerMgr.ActorAddress(ctx)
	if err != nil {
		return err
	}

	var deals []*types.MinerDeal
	for _, addr := range addrs {
		tdeals, err := p.dealStore.ListDeal(addr)
		if err != nil {
			return nil
		}
		deals = append(deals, tdeals...)
	}

	// Fire restart event on all active deals
	if err := p.restartDeals(ctx, deals); err != nil {
		return fmt.Errorf("failed to restart deals: %w", err)
	}
	return nil
}

func (p *StorageProviderV2Impl) restartDeals(ctx context.Context, deals []*types.MinerDeal) error {
	for _, deal := range deals {
		if deal.State == storagemarket.StorageDealSlashed || deal.State == storagemarket.StorageDealExpired || deal.State == storagemarket.StorageDealError {
			continue
		}

		err := p.dealProcess.HandleOff(ctx, deal)
		if err != nil {
			return err
		}
	}
	return nil
}

// Stop terminates processing of deals on a StorageProvider
func (p *StorageProviderV2Impl) Stop() error {
	p.unsubDataTransfer()

	return p.net.StopHandlingRequests()
}

// ImportDataForDeal manually imports data for an offline storage deal
// It will verify that the data in the passed io.Reader matches the expected piece
// cid for the given deal or it will error
func (p *StorageProviderV2Impl) ImportDataForDeal(ctx context.Context, propCid cid.Cid, data io.Reader) error {
	// TODO: be able to check if we have enough disk space
	d, err := p.dealStore.GetDeal(propCid)
	if err != nil {
		return xerrors.Errorf("failed getting deal %s: %w", propCid, err)
	}

	tempfi, err := p.fs.CreateTemp()
	if err != nil {
		return xerrors.Errorf("failed to create temp file for data import: %w", err)
	}
	defer tempfi.Close()
	cleanup := func() {
		_ = tempfi.Close()
		_ = p.fs.Delete(tempfi.Path())
	}

	log.Debugw("will copy imported file to local file", "propCid", propCid)
	n, err := io.Copy(tempfi, data)
	if err != nil {
		cleanup()
		return xerrors.Errorf("importing deal data failed: %w", err)
	}
	log.Debugw("finished copying imported file to local file", "propCid", propCid)

	_ = n // TODO: verify n?

	carSize := uint64(tempfi.Size())

	_, err = tempfi.Seek(0, io.SeekStart)
	if err != nil {
		cleanup()
		return xerrors.Errorf("failed to seek through temp imported file: %w", err)
	}

	proofType, err := p.spn.GetProofType(ctx, d.Proposal.Provider, nil) // TODO: 判断是不是属于此矿池?
	if err != nil {
		cleanup()
		return xerrors.Errorf("failed to determine proof type: %w", err)
	}
	log.Debugw("fetched proof type", "propCid", propCid)

	pieceCid, err := utils.GeneratePieceCommitment(proofType, tempfi, carSize)
	if err != nil {
		cleanup()
		return xerrors.Errorf("failed to generate commP: %w", err)
	}
	log.Debugw("generated pieceCid for imported file", "propCid", propCid)

	if carSizePadded := padreader.PaddedSize(carSize).Padded(); carSizePadded < d.Proposal.PieceSize {
		// need to pad up!
		rawPaddedCommp, err := commp.PadCommP(
			// we know how long a pieceCid "hash" is, just blindly extract the trailing 32 bytes
			pieceCid.Hash()[len(pieceCid.Hash())-32:],
			uint64(carSizePadded),
			uint64(d.Proposal.PieceSize),
		)
		if err != nil {
			cleanup()
			return err
		}
		pieceCid, _ = commcid.DataCommitmentV1ToCID(rawPaddedCommp)
	}

	// Verify CommP matches
	if !pieceCid.Equals(d.Proposal.PieceCID) {
		cleanup()
		return xerrors.Errorf("given data does not match expected commP (got: %s, expected %s)", pieceCid, d.Proposal.PieceCID)
	}

	log.Debugw("will fire ProviderEventVerifiedData for imported file", "propCid", propCid)

	// TODO: 实现 ProviderEventVerifiedData 后的状态机逻辑
	d.PiecePath = filestore.Path("")
	d.MetadataPath = tempfi.Path()

	d.State = storagemarket.StorageDealReserveProviderFunds

	return p.dealProcess.HandleOff(ctx, d)
}

// GetAsk returns the storage miner's ask, or nil if one does not exist.
func (p *StorageProviderV2Impl) GetAsk(mAddr address.Address) (*storagemarket.SignedStorageAsk, error) {
	return p.storedAsk.GetAsk(mAddr)
}

// AddStorageCollateral adds storage collateral
func (p *StorageProviderV2Impl) AddStorageCollateral(ctx context.Context, mAddr address.Address, amount abi.TokenAmount) error {
	done := make(chan error, 1)

	mcid, err := p.spn.AddFunds(ctx, mAddr, amount)
	if err != nil {
		return err
	}

	err = p.spn.WaitForMessage(ctx, mcid, func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error {
		if err != nil {
			done <- xerrors.Errorf("AddFunds errored: %w", err)
		} else if code != exitcode.Ok {
			done <- xerrors.Errorf("AddFunds error, exit code: %s", code.String())
		} else {
			done <- nil
		}
		return nil
	})

	if err != nil {
		return err
	}

	return <-done
}

// GetStorageCollateral returns the current collateral balance
func (p *StorageProviderV2Impl) GetStorageCollateral(ctx context.Context, mAddr address.Address) (storagemarket.Balance, error) {
	tok, _, err := p.spn.GetChainHead(ctx)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	return p.spn.GetBalance(ctx, mAddr, tok)
}

// ListLocalDeals lists deals processed by this storage provider
func (p *StorageProviderV2Impl) ListLocalDeals(mAddr address.Address) ([]storagemarket.MinerDeal, error) {
	deals, err := p.dealStore.ListDeal(mAddr)
	if err != nil {
		return nil, err
	}

	resDeals := make([]storagemarket.MinerDeal, len(deals))
	for idx, deal := range deals {
		resDeals[idx] = *deal.FilMarketMinerDeal()
	}

	return resDeals, nil
}

// SetAsk configures the storage miner's ask with the provided price,
// duration, and options. Any previously-existing ask is replaced.
func (p *StorageProviderV2Impl) SetAsk(mAddr address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error {
	return p.storedAsk.SetAsk(mAddr, price, verifiedPrice, duration, options...)
}

// SubscribeToEvents allows another component to listen for events on the StorageProvider
// in order to track deals as they progress through the deal flow
func (p *StorageProviderV2Impl) SubscribeToEvents(subscriber storagemarket.ProviderSubscriber) shared.Unsubscribe {
	return shared.Unsubscribe(p.pubSub.Subscribe(subscriber))
}

func curTime() cbg.CborTime {
	now := time.Now()
	return cbg.CborTime(time.Unix(0, now.UnixNano()).UTC())
}
