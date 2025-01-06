package storageprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs-force-community/metrics"
	"go.uber.org/fx"

	"github.com/hannahhoward/go-pubsub"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"

	"github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/minermgr"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	types3 "github.com/ipfs-force-community/droplet/v2/types"
	"github.com/ipfs-force-community/droplet/v2/utils"

	types2 "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

type internalProviderEvent struct {
	evt  storagemarket.ProviderEvent
	deal *types.MinerDeal
}

// ProviderSubscriber is a callback that is run when events are emitted on a StorageProvider
type ProviderSubscriber func(event storagemarket.ProviderEvent, deal *types.MinerDeal)

func providerDispatcher(evt pubsub.Event, fn pubsub.SubscriberFn) error {
	ie, ok := evt.(internalProviderEvent)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb, ok := fn.(ProviderSubscriber)
	if !ok {
		return errors.New("wrong type of callback")
	}
	cb(ie.evt, ie.deal)
	return nil
}

type EventPublishAdapter struct {
	dealStore repo.StorageDealRepo
	Pubsub    *pubsub.PubSub
}

func NewEventPublishAdapter(repo repo.Repo) *EventPublishAdapter {
	return &EventPublishAdapter{dealStore: repo.StorageDealRepo(), Pubsub: pubsub.New(providerDispatcher)}
}

func (p *EventPublishAdapter) Publish(evt storagemarket.ProviderEvent, deal *types.MinerDeal) {
	err := p.Pubsub.Publish(internalProviderEvent{evt: evt, deal: deal})
	if err != nil {
		log.Debugf("publish deal %s event %s err: %s", deal.ProposalCid, evt, err)
	}
}

func (p *EventPublishAdapter) PublishWithCid(evt storagemarket.ProviderEvent, cid cid.Cid) {
	deal, err := p.dealStore.GetDeal(context.TODO(), cid)
	if err != nil {
		log.Debugf("get deal fail %s  when publish event %s err: %s", cid, evt, err)
		return
	}
	err = p.Pubsub.Publish(internalProviderEvent{evt: evt, deal: deal})
	if err != nil {
		log.Debugf("publish deal %s event %s err: %s", cid, evt, err)
	}
}

// StorageProvider provides an interface to the storage market for a single
// storage miner.
type StorageProvider interface {
	// Start initializes deal processing on a StorageProvider and restarts in progress deals.
	// It also registers the provider with a StorageMarketNetwork so it can receive incoming
	// messages on the storage market's libp2p protocols
	Start(ctx context.Context) error

	// Stop terminates processing of deals on a StorageProvider
	Stop() error

	// AddStorageCollateral adds storage collateral
	AddStorageCollateral(ctx context.Context, mAddr address.Address, amount abi.TokenAmount) error

	// GetStorageCollateral returns the current collateral balance
	GetStorageCollateral(ctx context.Context, mAddr address.Address) (storagemarket.Balance, error)

	// ImportDataForDeals manually batch imports data for an offline storage deals
	ImportDataForDeals(ctx context.Context, refs []*types.ImportDataRef, skipCommP bool) ([]*types.ImportDataResult, error)

	// ImportPublishedDeal manually import published deals to storage deals
	ImportPublishedDeal(ctx context.Context, deal types.MinerDeal) error

	// ImportDeals manually import deals to storage deals
	ImportDeals(ctx context.Context, deal map[address.Address][]*types.MinerDeal) error

	// SubscribeToEvents listens for events that happen related to storage deals on a provider
	SubscribeToEvents(subscriber ProviderSubscriber) shared.Unsubscribe
}

type StorageProviderImpl struct {
	ctx context.Context

	net smnet.StorageMarketNetwork

	tf config.TransferFileStoreConfigFunc

	host host.Host

	spn       StorageProviderNode
	conns     *connmanager.ConnManager
	storedAsk IStorageAsk

	eventPublisher *EventPublishAdapter

	unsubDataTransfer datatransfer.Unsubscribe

	dealStore         repo.StorageDealRepo
	dealProcess       StorageDealHandler
	transferProcess   IDatatransferHandler
	storageDealStream *StorageDealStream
	minerMgr          minermgr.IMinerMgr
	pieceStorageMgr   *piecestorage.PieceStorageManager
}

// NewStorageProvider returns a new storage provider
func NewStorageProvider(
	mCtx metrics.MetricsCtx,
	lc fx.Lifecycle,
	storedAsk IStorageAsk,
	h host.Host,
	tf config.TransferFileStoreConfigFunc,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	dataTransfer network.ProviderDataTransfer,
	spn StorageProviderNode,
	dagStore stores.DAGStoreWrapper,
	repo repo.Repo,
	minerMgr minermgr.IMinerMgr,
	mixMsgClient clients.IMixMessage,
	sdf config.StorageDealFilter,
	pb *EventPublishAdapter,
) (StorageProvider, error) {
	net := smnet.NewFromLibp2pHost(h)

	spV2 := &StorageProviderImpl{
		ctx: metrics.LifecycleCtx(mCtx, lc),

		host: h,

		net: net,

		tf: tf,

		spn:       spn,
		conns:     connmanager.NewConnManager(),
		storedAsk: storedAsk,

		eventPublisher: pb,

		dealStore: repo.StorageDealRepo(),

		minerMgr:        minerMgr,
		pieceStorageMgr: pieceStorageMgr,
	}

	dealProcess, err := NewStorageDealProcessImpl(mCtx, spV2.conns, newPeerTagger(spV2.net), spV2.spn, spV2.dealStore, spV2.storedAsk, tf, minerMgr, pieceStorageMgr, dataTransfer, dagStore, sdf, pb)
	if err != nil {
		return nil, err
	}
	spV2.dealProcess = dealProcess

	spV2.transferProcess = NewDataTransferProcess(dealProcess, spV2.dealStore)
	// register a data transfer event handler -- this will send events to the state machines based on DT events
	spV2.unsubDataTransfer = dataTransfer.SubscribeToEvents(ProviderDataTransferSubscriber(spV2.transferProcess, pb)) // fsm.Group

	storageDealStream, err := NewStorageDealStream(spV2.conns, spV2.storedAsk, spV2.spn, spV2.dealStore, spV2.net, tf, dealProcess, mixMsgClient, pb)
	if err != nil {
		return nil, err
	}
	spV2.storageDealStream = storageDealStream

	return spV2, nil
}

// Start initializes deal processing on a StorageProvider and restarts in progress deals.
// It also registers the provider with a StorageMarketNetwork so it can receive incoming
// messages on the storage market's libp2p protocols
func (p *StorageProviderImpl) Start(ctx context.Context) error {
	err := p.net.SetDelegate(p.storageDealStream)
	if err != nil {
		return err
	}

	p.host.SetStreamHandler(types3.DealProtocolv120ID, p.storageDealStream.HandleNewDealStream)
	p.host.SetStreamHandler(types3.DealProtocolv121ID, p.storageDealStream.HandleNewDealStream)
	p.host.SetStreamHandler(types3.DealStatusV12ProtocolID, p.storageDealStream.HandleNewDealStatusStream)

	go func() {
		err := p.start(ctx)
		if err != nil {
			log.Error(err.Error())
		}
	}()

	return nil
}

func (p *StorageProviderImpl) start(ctx context.Context) error {
	// Run datastore and DAG store migrations
	deals, err := p.dealStore.ListDeal(ctx, &types.StorageDealQueryParams{Page: types.Page{Limit: math.MaxInt32}})
	if err != nil {
		return fmt.Errorf("failed to list deals: %w", err)
	}
	// Fire restart event on all active deals
	if err := p.restartDeals(ctx, deals); err != nil {
		return fmt.Errorf("failed to restart deals: %w", err)
	}
	return nil
}

func IsTerminateState(state storagemarket.StorageDealStatus) bool {
	if state == storagemarket.StorageDealSlashed || state == storagemarket.StorageDealExpired ||
		state == storagemarket.StorageDealError || state == storagemarket.StorageDealFailing {
		return true
	}

	return false
}

func (p *StorageProviderImpl) restartDeals(ctx context.Context, deals []*types.MinerDeal) error {
	for _, deal := range deals {
		if IsTerminateState(deal.State) {
			continue
		}

		go func(deal *types.MinerDeal) {
			err := p.dealProcess.HandleOff(ctx, deal)
			if err != nil {
				log.Errorf("deal %s handle off err: %s", deal.ProposalCid, err)
			}
		}(deal)
	}
	return nil
}

// Stop terminates processing of deals on a StorageProvider
func (p *StorageProviderImpl) Stop() error {
	p.unsubDataTransfer()

	p.host.RemoveStreamHandler(types3.DealProtocolv120ID)
	p.host.RemoveStreamHandler(types3.DealProtocolv121ID)
	p.host.RemoveStreamHandler(types3.DealStatusV12ProtocolID)

	return p.net.StopHandlingRequests()
}

func GetDealByDataRef(ctx context.Context, r repo.StorageDealRepo, ref *types.ImportDataRef) (*types.MinerDeal, string, error) {
	deal, err := r.GetDeal(ctx, ref.ProposalCID)
	if err == nil {
		return deal, ref.ProposalCID.String(), nil
	}
	deal, err = r.GetDealByUUID(ctx, ref.UUID)
	return deal, ref.UUID.String(), err
}

// ImportDataForDeals manually batch imports data for offline storage deals
func (p *StorageProviderImpl) ImportDataForDeals(ctx context.Context, refs []*types.ImportDataRef, skipCommP bool) ([]*types.ImportDataResult, error) {
	// TODO: be able to check if we have enough disk space
	results := make([]*types.ImportDataResult, 0, len(refs))
	minerDeals := make(map[address.Address][]*types.MinerDeal)
	targets := make(map[cid.Cid]string, len(refs))
	log.Debugf("importing data for %d deals, skipCommP: %v", len(refs), skipCommP)
	for _, ref := range refs {
		d, target, err := GetDealByDataRef(ctx, p.dealStore, ref)
		if err != nil {
			results = append(results, &types.ImportDataResult{
				Target:  target,
				Message: fmt.Errorf("failed getting deal: %v", err).Error(),
			})
			continue
		}
		if err := p.importDataForDeal(ctx, d, ref, skipCommP); err != nil {
			results = append(results, &types.ImportDataResult{
				Target:  target,
				Message: err.Error(),
			})
			continue
		}
		targets[d.ProposalCid] = target
		minerDeals[d.Proposal.Provider] = append(minerDeals[d.Proposal.Provider], d)
	}

	for provider, deals := range minerDeals {
		res, err := p.batchReserverFunds(p.ctx, deals)
		if err != nil {
			log.Errorf("batch reserver funds for %s failed: %v", provider, err)
			for _, deal := range deals {
				results = append(results, &types.ImportDataResult{
					Target:  targets[deal.ProposalCid],
					Message: err.Error(),
				})
			}
			continue
		}

		for _, deal := range deals {
			if err := res[deal.ProposalCid]; err != nil {
				results = append(results, &types.ImportDataResult{
					Target:  targets[deal.ProposalCid],
					Message: err.Error(),
				})
				continue
			}
			results = append(results, &types.ImportDataResult{
				Target: targets[deal.ProposalCid],
			})

			go func(deal *types.MinerDeal) {
				err := p.dealProcess.HandleOff(p.ctx, deal)
				if err != nil {
					log.Errorf("deal %s handle off err: %s", deal.ProposalCid, err)
				}
			}(deal)
		}
	}

	return results, nil
}

func (p *StorageProviderImpl) importDataForDeal(ctx context.Context, d *types.MinerDeal, ref *types.ImportDataRef, skipCommP bool) error {
	propCid := d.ProposalCid
	if IsTerminateState(d.State) {
		return fmt.Errorf("deal %s is terminate state", propCid)
	}

	if d.State > storagemarket.StorageDealWaitingForData {
		return fmt.Errorf("deal %s does not support offline data", propCid)
	}

	var r io.Reader
	var carSize int64
	var piecePath filestore.Path
	var cleanup = func() {}

	pieceStore, err := p.pieceStorageMgr.FindStorageForRead(ctx, d.Proposal.PieceCID.String())
	if err == nil {
		log.Debugf("found %v already in piece storage", d.Proposal.PieceCID)

		// In order to avoid errors in the deal, the files in the piece store were deleted.
		piecePath = filestore.Path("")
		if carSize, err = pieceStore.Len(ctx, d.Proposal.PieceCID.String()); err != nil {
			return fmt.Errorf("got piece size from piece store failed: %v", err)
		}
		readerCloser, err := pieceStore.GetReaderCloser(ctx, d.Proposal.PieceCID.String())
		if err != nil {
			return fmt.Errorf("got reader from piece store failed: %v", err)
		}
		r = readerCloser

		defer func() {
			if err = readerCloser.Close(); err != nil {
				log.Errorf("unable to close piece storage: %v, %v", d.Proposal.PieceCID, err)
			}
		}()
	} else {
		log.Debugf("not found %s in piece storage", d.Proposal.PieceCID)

		data, err := os.Open(ref.File)
		if err != nil {
			return err
		}

		fs, err := p.tf(d.Proposal.Provider)
		if err != nil {
			return fmt.Errorf("failed to create temp filestore for provider %s: %w", d.Proposal.Provider.String(), err)
		}

		tempfi, err := fs.CreateTemp()
		if err != nil {
			return fmt.Errorf("failed to create temp file for data import: %w", err)
		}
		defer func() {
			if err := tempfi.Close(); err != nil {
				log.Errorf("unable to close stream %v", err)
			}
		}()
		cleanup = func() {
			_ = tempfi.Close()
			_ = fs.Delete(tempfi.Path())
		}

		log.Debugw("will copy imported file to local file", "propCid", propCid)
		n, err := io.Copy(tempfi, data)
		if err != nil {
			cleanup()
			return fmt.Errorf("importing deal data failed: %w", err)
		}
		log.Debugw("finished copying imported file to local file", "propCid", propCid)

		_ = n // TODO: verify n?

		carSize = tempfi.Size()
		piecePath = tempfi.Path()
		_, err = tempfi.Seek(0, io.SeekStart)
		if err != nil {
			cleanup()
			return fmt.Errorf("failed to seek through temp imported file: %w", err)
		}

		r = tempfi
	}

	if !skipCommP {
		log.Debugf("will calculate piece cid")

		proofType, err := p.spn.GetProofType(ctx, d.Proposal.Provider, nil) // TODO: 判断是不是属于此矿池?
		if err != nil {
			p.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, d)
			cleanup()
			return fmt.Errorf("failed to determine proof type: %w", err)
		}
		log.Debugw("fetched proof type", "propCid", propCid)

		pieceCid, err := utils.GeneratePieceCommitment(proofType, r, uint64(carSize))
		if err != nil {
			cleanup()
			return fmt.Errorf("failed to generate commP: %w", err)
		}
		if carSizePadded := padreader.PaddedSize(uint64(carSize)).Padded(); carSizePadded < d.Proposal.PieceSize {
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
			return fmt.Errorf("given data does not match expected commP (got: %s, expected %s)", pieceCid, d.Proposal.PieceCID)
		}
	}

	log.Debugw("will fire ReserveProviderFunds for imported file", "propCid", propCid)

	// "will fire VerifiedData for imported file
	d.PiecePath = piecePath
	d.MetadataPath = filestore.Path("")
	log.Infof("deal %s piece path: %s, payload size: %v", propCid, d.PiecePath, carSize)

	d.State = storagemarket.StorageDealReserveProviderFunds
	d.PieceStatus = types.Undefine
	if err := p.dealStore.SaveDeal(ctx, d); err != nil {
		return fmt.Errorf("save deal(%d) failed:%w", d.DealID, err)
	}

	p.eventPublisher.Publish(storagemarket.ProviderEventManualDataReceived, d)

	return nil
}

// batchReserverFunds batch reserver funds for deals
func (p *StorageProviderImpl) batchReserverFunds(ctx context.Context, deals []*types.MinerDeal) (map[cid.Cid]error, error) {
	handleError := func(deals []*types.MinerDeal, evt storagemarket.ProviderEvent, err error) (map[cid.Cid]error, error) {
		for _, deal := range deals {
			p.eventPublisher.Publish(evt, deal)
			_ = p.dealProcess.HandleError(ctx, deal, err)
		}
		return nil, err
	}

	res := make(map[cid.Cid]error, len(deals))
	provider := deals[0].Proposal.Provider
	allFunds := big.NewInt(0)
	fundList := make([]big.Int, 0, len(deals))
	for _, d := range deals {
		big.Add(allFunds, d.Proposal.ProviderCollateral)
		fundList = append(fundList, d.Proposal.ProviderCollateral)
	}

	tok, _, err := p.spn.GetChainHead(ctx)
	if err != nil {
		return handleError(deals, storagemarket.ProviderEventNodeErrored, fmt.Errorf("acquiring chain head: %v", err))
	}

	waddr, err := p.spn.GetMinerWorkerAddress(ctx, provider, tok)
	if err != nil {
		return handleError(deals, storagemarket.ProviderEventNodeErrored, fmt.Errorf("looking up miner worker: %v", err))
	}

	mcid, err := p.spn.ReserveFunds(ctx, waddr, provider, allFunds)
	if err != nil {
		return handleError(deals, storagemarket.ProviderEventNodeErrored, fmt.Errorf("reserving funds: %v", err))
	}

	for i, deal := range deals {
		p.eventPublisher.Publish(storagemarket.ProviderEventFundsReserved, deal)
		res[deal.ProposalCid] = nil

		if deal.FundsReserved.Nil() {
			deal.FundsReserved = fundList[i]
		} else {
			deal.FundsReserved = big.Add(deal.FundsReserved, fundList[i])
		}

		// if no message was sent, and there was no error, funds were already available
		if mcid != cid.Undef {
			deal.AddFundsCid = &mcid
			deal.State = storagemarket.StorageDealProviderFunding
		} else {
			p.eventPublisher.Publish(storagemarket.ProviderEventFunded, deal)
			deal.State = storagemarket.StorageDealPublish // PublishDeal
		}

		p.eventPublisher.Publish(storagemarket.ProviderEventFundingInitiated, deal)
		err = p.dealStore.SaveDeal(ctx, deal)
		if err != nil {
			_ = p.dealProcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database: %v", err))
			res[deal.ProposalCid] = err
		}
	}

	return res, nil
}

// ImportPublishedDeal manually import published deals for an storage deal
// It will verify that the deal is actually online
func (p *StorageProviderImpl) ImportPublishedDeal(ctx context.Context, deal types.MinerDeal) error {
	// check if exit
	if !p.minerMgr.Has(ctx, deal.Proposal.Provider) {
		return fmt.Errorf("miner %s not support", deal.Proposal.Provider)
	}

	// confirm deal proposal in params is correct
	dealPCid, err := deal.ClientDealProposal.Proposal.Cid()
	if err != nil {
		return fmt.Errorf("unable to get proposal cid from deal online %w", err)
	}
	if dealPCid != deal.ProposalCid {
		return fmt.Errorf("deal proposal(%s) not match the calculated result(%s)", deal.ProposalCid, dealPCid)
	}

	// check is deal online
	onlineDeal, err := p.spn.StateMarketStorageDeal(ctx, deal.DealID, types2.EmptyTSK)
	if err != nil {
		p.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, &deal)
		return fmt.Errorf("cannt find deal(%d) ", deal.DealID)
	}
	// get client addr
	clientAccount := onlineDeal.Proposal.Client
	if deal.Proposal.Client != clientAccount {
		switch deal.Proposal.Client.Protocol() {
		case address.BLS, address.SECP256K1:
			clientAccount, err = p.spn.StateAccountKey(ctx, onlineDeal.Proposal.Client, types2.EmptyTSK)
		case address.Actor:
			clientAccount, err = p.spn.StateLookupID(ctx, onlineDeal.Proposal.Client, types2.EmptyTSK)
		}
	}
	if err != nil {
		p.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, &deal)
		return fmt.Errorf("get account for %s err: %w", onlineDeal.Proposal.Client, err)
	}
	// change DealProposal the same as type in spec-actors
	onlineProposal := types2.DealProposal{
		PieceCID:             onlineDeal.Proposal.PieceCID,
		PieceSize:            onlineDeal.Proposal.PieceSize,
		VerifiedDeal:         onlineDeal.Proposal.VerifiedDeal,
		Client:               clientAccount,
		Provider:             onlineDeal.Proposal.Provider,
		Label:                onlineDeal.Proposal.Label,
		StartEpoch:           onlineDeal.Proposal.StartEpoch,
		EndEpoch:             onlineDeal.Proposal.EndEpoch,
		StoragePricePerEpoch: onlineDeal.Proposal.StoragePricePerEpoch,
		ProviderCollateral:   onlineDeal.Proposal.ProviderCollateral,
		ClientCollateral:     onlineDeal.Proposal.ClientCollateral,
	}
	pCid, err := onlineProposal.Cid()
	if err != nil {
		return fmt.Errorf("fail build cid %w", err)
	}
	if pCid != dealPCid {
		log.Errorf("online: %v, rpc receive: %v", onlineDeal.Proposal, deal.ClientDealProposal.Proposal)
		return fmt.Errorf("deal online proposal(%s) not match with proposal(%s)", pCid, dealPCid)
	}

	// check if local exit
	if _, err := p.dealStore.GetDeal(ctx, deal.ProposalCid); err == nil {
		return fmt.Errorf("deal exist proposal cid %s id %d", deal.ProposalCid, deal.DealID)
	}

	improtDeal := &types.MinerDeal{
		ClientDealProposal: deal.ClientDealProposal, // checked
		ProposalCid:        deal.ProposalCid,        // checked
		PublishCid:         deal.PublishCid,         // unable to check, msg maybe unable found
		Client:             deal.Client,             // not necessary
		PayloadSize:        deal.PayloadSize,        // unable to check
		Ref: &storagemarket.DataRef{
			TransferType: "import",
			Root:         deal.Ref.Root, // unable to check
			PieceCid:     &deal.Proposal.PieceCID,
			PieceSize:    deal.Proposal.PieceSize.Unpadded(),
			RawBlockSize: deal.PayloadSize,
		},
		AvailableForRetrieval: deal.AvailableForRetrieval,
		DealID:                deal.DealID,
		// default
		AddFundsCid:       nil,
		Miner:             p.net.ID(),
		State:             storagemarket.StorageDealAwaitingPreCommit,
		PiecePath:         "",
		MetadataPath:      "",
		SlashEpoch:        0,
		FastRetrieval:     true,
		Message:           "",
		FundsReserved:     abi.TokenAmount{},
		CreationTime:      cbg.CborTime(time.Now()),
		TransferChannelID: nil,
		SectorNumber:      0,
		Offset:            0,
		PieceStatus:       types.Undefine,
		InboundCAR:        "",
	}
	return p.dealStore.SaveDeal(ctx, improtDeal)
}

// ImportDeals manually import deals
func (p *StorageProviderImpl) ImportDeals(ctx context.Context, deals map[address.Address][]*types.MinerDeal) error {
	tok, _, err := p.spn.GetChainHead(ctx)
	if err != nil {
		return fmt.Errorf("node error getting most recent state id: %w", err)
	}

	var errs *multierror.Error
	batchSize := 100
	for provider, d := range deals {
		pendingDeals := make([]*types.MinerDeal, 0, len(d))

		if !p.minerMgr.Has(ctx, provider) {
			errs = multierror.Append(errs, fmt.Errorf("miner %s not support", provider))
			continue
		}

		pubDeals, err := p.dealStore.GetDeals(ctx, provider, 0, math.MaxInt64)
		if err != nil {
			return fmt.Errorf("get deal by provider %s failed: %s", provider, err)
		}
		proposalCIDs := make(map[cid.Cid]struct{}, len(pubDeals))
		for _, d := range pubDeals {
			proposalCIDs[d.ProposalCid] = struct{}{}
		}

		for _, deal := range d {
			_, ok := proposalCIDs[deal.ProposalCid]
			if ok {
				errs = multierror.Append(errs, fmt.Errorf("deal exist: %s", deal.ProposalCid))
				continue
			}
			proposalCIDs[deal.ProposalCid] = struct{}{}

			// todo: The ClientSignature of the boost deal cannot be obtained currently
			if len(deal.ClientDealProposal.ClientSignature.Data) != 0 {
				err := providerutils.VerifyProposal(ctx, deal.ClientDealProposal, tok, p.spn.VerifySignature)
				if err != nil {
					errs = multierror.Append(errs, fmt.Errorf("verify %s proposal failed: %v", deal.ProposalCid, err))
					continue
				}
			}
			pendingDeals = append(pendingDeals, deal)
			if len(pendingDeals) == batchSize {
				if err = p.dealStore.CreateDeals(ctx, pendingDeals); err != nil {
					return fmt.Errorf("save miner %s deal to database %v", provider, err)
				}
				pendingDeals = pendingDeals[:0]
			}
		}

		if err = p.dealStore.CreateDeals(ctx, pendingDeals); err != nil {
			return fmt.Errorf("save miner %s deal to database %v", provider, err)
		}
	}

	return errs.ErrorOrNil()
}

// AddStorageCollateral adds storage collateral
func (p *StorageProviderImpl) AddStorageCollateral(ctx context.Context, mAddr address.Address, amount abi.TokenAmount) error {
	done := make(chan error, 1)

	mcid, err := p.spn.AddFunds(ctx, mAddr, amount)
	if err != nil {
		return err
	}

	err = p.spn.WaitForMessage(ctx, mcid, func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error {
		if err != nil {
			done <- fmt.Errorf("AddFunds errored: %w", err)
		} else if code != exitcode.Ok {
			done <- fmt.Errorf("AddFunds error, exit code: %s", code.String())
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
func (p *StorageProviderImpl) GetStorageCollateral(ctx context.Context, mAddr address.Address) (storagemarket.Balance, error) {
	tok, _, err := p.spn.GetChainHead(ctx)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	return p.spn.GetBalance(ctx, mAddr, tok)
}

// SubscribeToEvents allows another component to listen for events on the StorageProvider
// in order to track deals as they progress through the deal flow
func (p *StorageProviderImpl) SubscribeToEvents(subscriber ProviderSubscriber) shared.Unsubscribe {
	return shared.Unsubscribe(p.eventPublisher.Pubsub.Subscribe(subscriber))
}

func curTime() cbg.CborTime {
	now := time.Now()
	return cbg.CborTime(time.Unix(0, now.UnixNano()).UTC())
}
