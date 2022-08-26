package storageprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/hannahhoward/go-pubsub"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/mitchellh/go-homedir"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"

	"github.com/filecoin-project/venus-market/v2/api/clients"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/minermgr"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	"github.com/filecoin-project/venus-market/v2/utils"

	"github.com/filecoin-project/go-state-types/builtin/v8/market"

	types2 "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

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

	// ImportDataForDeal manually imports data for an offline storage deal
	ImportDataForDeal(ctx context.Context, propCid cid.Cid, data io.Reader) error

	//ImportPublishedDeal manually import published deals to storage deals
	ImportPublishedDeal(ctx context.Context, deal types.MinerDeal) error

	//ImportOfflineDeal manually import offline deals to storage deals
	ImportOfflineDeal(ctx context.Context, deal types.MinerDeal) error

	// SubscribeToEvents listens for events that happen related to storage deals on a provider
	SubscribeToEvents(subscriber storagemarket.ProviderSubscriber) shared.Unsubscribe
}

type StorageProviderImpl struct {
	net smnet.StorageMarketNetwork

	spn       StorageProviderNode
	fs        filestore.FileStore
	conns     *connmanager.ConnManager
	storedAsk IStorageAsk

	pubSub *pubsub.PubSub

	unsubDataTransfer datatransfer.Unsubscribe

	dealStore       repo.StorageDealRepo
	dealProcess     StorageDealHandler
	transferProcess IDatatransferHandler
	storageReceiver smnet.StorageReceiver
	minerMgr        minermgr.IAddrMgr
}

type internalProviderEvent struct {
	evt  storagemarket.ProviderEvent
	deal storagemarket.MinerDeal
}

func providerDispatcher(evt pubsub.Event, fn pubsub.SubscriberFn) error {
	ie, ok := evt.(internalProviderEvent)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb, ok := fn.(storagemarket.ProviderSubscriber)
	if !ok {
		return errors.New("wrong type of callback")
	}
	cb(ie.evt, ie.deal)
	return nil
}

// NewStorageProvider returns a new storage provider
func NewStorageProvider(
	mCtx metrics.MetricsCtx,
	storedAsk IStorageAsk,
	h host.Host,
	cfg *config.MarketConfig,
	homeDir *config.HomeDir,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	dataTransfer network.ProviderDataTransfer,
	spn StorageProviderNode,
	dagStore stores.DAGStoreWrapper,
	repo repo.Repo,
	minerMgr minermgr.IAddrMgr,
	mixMsgClient clients.IMixMessage,
) (StorageProvider, error) {
	net := smnet.NewFromLibp2pHost(h)

	var err error
	transferPath := cfg.TransfePath
	if len(transferPath) == 0 {
		transferPath = string(*homeDir)
	}
	transferPath, err = homedir.Expand(transferPath)
	if err != nil {
		return nil, err
	}
	store, err := filestore.NewLocalFileStore(filestore.OsPath(transferPath))
	if err != nil {
		return nil, err
	}

	spV2 := &StorageProviderImpl{
		net: net,

		spn:       spn,
		fs:        store,
		conns:     connmanager.NewConnManager(),
		storedAsk: storedAsk,

		pubSub: pubsub.New(providerDispatcher),

		dealStore: repo.StorageDealRepo(),

		minerMgr: minerMgr,
	}

	dealProcess, err := NewStorageDealProcessImpl(mCtx, spV2.conns, newPeerTagger(spV2.net), spV2.spn, spV2.dealStore, spV2.storedAsk, spV2.fs, minerMgr, repo, pieceStorageMgr, dataTransfer, dagStore)
	if err != nil {
		return nil, err
	}
	spV2.dealProcess = dealProcess

	spV2.transferProcess = NewDataTransferProcess(dealProcess, spV2.dealStore)
	// register a data transfer event handler -- this will send events to the state machines based on DT events
	spV2.unsubDataTransfer = dataTransfer.SubscribeToEvents(ProviderDataTransferSubscriber(spV2.transferProcess)) // fsm.Group

	storageReceiver, err := NewStorageDealStream(spV2.conns, spV2.storedAsk, spV2.spn, spV2.dealStore, spV2.net, spV2.fs, dealProcess, mixMsgClient)
	if err != nil {
		return nil, err
	}
	spV2.storageReceiver = storageReceiver

	return spV2, nil
}

// Start initializes deal processing on a StorageProvider and restarts in progress deals.
// It also registers the provider with a StorageMarketNetwork so it can receive incoming
// messages on the storage market's libp2p protocols
func (p *StorageProviderImpl) Start(ctx context.Context) error {
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

func (p *StorageProviderImpl) start(ctx context.Context) error {
	// Run datastore and DAG store migrations
	deals, err := p.dealStore.ListDeal(ctx)
	if err != nil {
		return nil
	}
	// Fire restart event on all active deals
	if err := p.restartDeals(ctx, deals); err != nil {
		return fmt.Errorf("failed to restart deals: %w", err)
	}
	return nil
}

func isTerminateState(deal *types.MinerDeal) bool {
	if deal.State == storagemarket.StorageDealSlashed || deal.State == storagemarket.StorageDealExpired ||
		deal.State == storagemarket.StorageDealError || deal.State == storagemarket.StorageDealFailing {
		return true
	}

	return false
}

func (p *StorageProviderImpl) restartDeals(ctx context.Context, deals []*types.MinerDeal) error {
	for _, deal := range deals {
		if isTerminateState(deal) {
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

	return p.net.StopHandlingRequests()
}

// ImportDataForDeal manually imports data for an offline storage deal
// It will verify that the data in the passed io.Reader matches the expected piece
// cid for the given deal or it will error
func (p *StorageProviderImpl) ImportDataForDeal(ctx context.Context, propCid cid.Cid, data io.Reader) error {
	// TODO: be able to check if we have enough disk space
	d, err := p.dealStore.GetDeal(ctx, propCid)
	if err != nil {
		return fmt.Errorf("failed getting deal %s: %w", propCid, err)
	}

	// TODO: Check the deal status
	if isTerminateState(d) {
		return fmt.Errorf("deal %s is terminate state", propCid)
	}

	if d.State > storagemarket.StorageDealWaitingForData {
		return fmt.Errorf("deal %s does not support offline data", propCid)
	}

	tempfi, err := p.fs.CreateTemp()
	if err != nil {
		return fmt.Errorf("failed to create temp file for data import: %w", err)
	}
	defer func() {
		if err := tempfi.Close(); err != nil {
			log.Errorf("unable to close stream %v", err)
		}
	}()
	cleanup := func() {
		_ = tempfi.Close()
		_ = p.fs.Delete(tempfi.Path())
	}

	log.Debugw("will copy imported file to local file", "propCid", propCid)
	n, err := io.Copy(tempfi, data)
	if err != nil {
		cleanup()
		return fmt.Errorf("importing deal data failed: %w", err)
	}
	log.Debugw("finished copying imported file to local file", "propCid", propCid)

	_ = n // TODO: verify n?

	carSize := uint64(tempfi.Size())

	_, err = tempfi.Seek(0, io.SeekStart)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to seek through temp imported file: %w", err)
	}

	proofType, err := p.spn.GetProofType(ctx, d.Proposal.Provider, nil) // TODO: 判断是不是属于此矿池?
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to determine proof type: %w", err)
	}
	log.Debugw("fetched proof type", "propCid", propCid)

	pieceCid, err := utils.GeneratePieceCommitment(proofType, tempfi, carSize)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to generate commP: %w", err)
	}
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
		return fmt.Errorf("given data does not match expected commP (got: %s, expected %s)", pieceCid, d.Proposal.PieceCID)
	}

	log.Debugw("will fire ReserveProviderFunds for imported file", "propCid", propCid)

	// "will fire VerifiedData for imported file
	d.PiecePath = tempfi.Path()
	d.MetadataPath = filestore.Path("")
	log.Infof("deal %s piece path: %s", propCid, d.PiecePath)

	d.State = storagemarket.StorageDealReserveProviderFunds
	d.PieceStatus = types.Undefine
	if err := p.dealStore.SaveDeal(ctx, d); err != nil {
		return fmt.Errorf("save deal(%d) failed:%w", d.DealID, err)
	}
	go func() {
		err := p.dealProcess.HandleOff(context.TODO(), d)
		if err != nil {
			log.Errorf("deal %s handle off err: %s", propCid, err)
		}
	}()
	return nil
}

// ImportPublishedDeal manually import published deals for an storage deal
// It will verify that the deal is actually online
func (p *StorageProviderImpl) ImportPublishedDeal(ctx context.Context, deal types.MinerDeal) error {
	//check if exit
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
		return fmt.Errorf("get account for %s err: %w", onlineDeal.Proposal.Client, err)
	}
	// change DealProposal the same as type in spec-actors
	onlineProposal := market.DealProposal{
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

	//check if local exit
	if _, err := p.dealStore.GetDeal(ctx, deal.ProposalCid); err == nil {
		return fmt.Errorf("deal exist proposal cid %s id %d", deal.ProposalCid, deal.DealID)
	}

	improtDeal := &types.MinerDeal{
		ClientDealProposal: deal.ClientDealProposal, //checked
		ProposalCid:        deal.ProposalCid,        //checked
		PublishCid:         deal.PublishCid,         //unable to check, msg maybe unable found
		Client:             deal.Client,             //not necessary
		PayloadSize:        deal.PayloadSize,        //unable to check
		Ref: &storagemarket.DataRef{
			TransferType: "import",
			Root:         deal.Ref.Root, //unable to check
			PieceCid:     &deal.Proposal.PieceCID,
			PieceSize:    deal.Proposal.PieceSize.Unpadded(),
			RawBlockSize: deal.PayloadSize,
		},
		AvailableForRetrieval: deal.AvailableForRetrieval,
		DealID:                deal.DealID,
		//default
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

// ImportPublishedDeal manually import published deals for an storage deal
func (p *StorageProviderImpl) ImportOfflineDeal(ctx context.Context, deal types.MinerDeal) error {
	// check deal state
	if deal.State != storagemarket.StorageDealWaitingForData {
		return fmt.Errorf("deal state %s not match %s", storagemarket.DealStates[deal.State], storagemarket.DealStates[storagemarket.StorageDealWaitingForData])
	}

	// check transfer type
	if deal.Ref.TransferType != storagemarket.TTManual {
		return fmt.Errorf("transfer type %s not match %s", deal.Ref.TransferType, storagemarket.TTManual)
	}

	//check if miner exit
	if !p.minerMgr.Has(ctx, deal.Proposal.Provider) {
		return fmt.Errorf("miner %s not support", deal.Proposal.Provider)
	}

	//check if local exit the deal
	if _, err := p.dealStore.GetDeal(ctx, deal.ProposalCid); err == nil {
		return fmt.Errorf("deal exist proposal cid %s id %d", deal.ProposalCid, deal.DealID)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return err
	}

	// check client signature
	tok, _, err := p.spn.GetChainHead(ctx)
	if err != nil {
		return fmt.Errorf("node error getting most recent state id: %w", err)
	}

	if err := providerutils.VerifyProposal(ctx, deal.ClientDealProposal, tok, p.spn.VerifySignature); err != nil {
		return fmt.Errorf("verifying StorageDealProposal: %w", err)
	}

	err = p.dealStore.SaveDeal(ctx, &deal)
	if err != nil {
		return fmt.Errorf("save miner deal to database %w", err)
	}

	return nil
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
func (p *StorageProviderImpl) SubscribeToEvents(subscriber storagemarket.ProviderSubscriber) shared.Unsubscribe {
	return shared.Unsubscribe(p.pubSub.Subscribe(subscriber))
}

func curTime() cbg.CborTime {
	now := time.Now()
	return cbg.CborTime(time.Unix(0, now.UnixNano()).UTC())
}
