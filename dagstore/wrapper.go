package dagstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	levelds "github.com/ipfs/go-ds-leveldb"
	measure "github.com/ipfs/go-ds-measure"
	logging "github.com/ipfs/go-log/v2"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/metrics"
	"github.com/ipfs-force-community/droplet/v2/models/badger"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	carindex "github.com/ipld/go-car/v2/index"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerstates"
	"github.com/filecoin-project/go-fil-markets/stores"
)

const (
	maxRecoverAttempts = 1
	shardRegMarker     = ".shard-registration-complete"
)

var log = logging.Logger("dagstore")

type Wrapper struct {
	ctx          context.Context
	cancel       context.CancelFunc
	backgroundWg sync.WaitGroup

	cfg        *config.DAGStoreConfig
	dagst      dagstore.Interface
	minerAPI   MarketAPI
	failureCh  chan dagstore.ShardResult
	gcInterval time.Duration
}

var _ stores.DAGStoreWrapper = (*Wrapper)(nil)

func NewDAGStore(ctx context.Context,
	cfg *config.DAGStoreConfig,
	marketApi MarketAPI,
	repo repo.Repo,
) (*dagstore.DAGStore, *Wrapper, error) {
	// construct the DAG Store.
	registry := mount.NewRegistry()
	if err := registry.Register(marketScheme, mountTemplate(marketApi, cfg.UseTransient)); err != nil {
		return nil, nil, fmt.Errorf("failed to create registry: %w", err)
	}

	// The dagstore will write Shard failures to the `failureCh` here.
	failureCh := make(chan dagstore.ShardResult, 1)

	var (
		transientsDir = filepath.Join(cfg.RootDir, "transients")
		datastoreDir  = filepath.Join(cfg.RootDir, "datastore")
		indexDir      = filepath.Join(cfg.RootDir, "index")
	)

	if len(cfg.Transient) != 0 {
		transientsDir = cfg.Transient
	}

	if len(cfg.Index) != 0 {
		indexDir = cfg.Index
	}

	dstore, err := newDatastore(datastoreDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dagstore datastore in %s: %w", datastoreDir, err)
	}

	var shardRepo dagstore.ShardRepo
	if _, ok := repo.ShardRepo().(*badger.Shard); !ok {
		// store shard state to mysql
		shardRepo = repo.ShardRepo()
	} else {
		shardRepo = dagstore.NewBadgerShardRepo(dstore)
	}

	irepo, err := index.NewFSRepo(indexDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialise dagstore index repo")
	}

	dCfg := dagstore.Config{
		TransientsDir: transientsDir,
		IndexRepo:     irepo,
		ShardRepo:     shardRepo,
		MountRegistry: registry,
		FailureCh:     failureCh,
		// not limiting fetches globally, as the Lotus mount does
		// conditional throttling.
		MaxConcurrentIndex:        cfg.MaxConcurrentIndex,
		MaxConcurrentReadyFetches: cfg.MaxConcurrentReadyFetches,
		RecoverOnStart:            dagstore.RecoverOnAcquire,
	}

	if cfg.MongoTopIndex != nil && len(cfg.MongoTopIndex.Url) != 0 {
		dCfg.TopLevelIndex, err = NewMongoTopIndex(ctx, cfg.MongoTopIndex.Url)
		if err != nil {
			return nil, nil, err
		}
	} else {
		dCfg.TopLevelIndex = index.NewInverted(dstore)
	}

	dagst, err := dagstore.NewDAGStore(dCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create DAG store: %w", err)
	}

	// thread for metrics
	go func() {
		tick := time.NewTicker(1 * time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				infos := dagst.AllShardsInfo()
				stateCount := make(map[dagstore.ShardState]int)
				for _, info := range infos {
					if _, ok := stateCount[info.ShardState]; !ok {
						stateCount[info.ShardState] = 0
					}
					stateCount[info.ShardState]++
				}
				for state, count := range stateCount {
					metrics.ShardNum.Set(ctx, state.String(), int64(count))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	w := &Wrapper{
		cfg:        cfg,
		dagst:      dagst,
		minerAPI:   marketApi,
		failureCh:  failureCh,
		gcInterval: time.Duration(cfg.GCInterval),
	}

	if !cfg.UseTransient && cfg.GCInterval != 0 {
		w.gcInterval = 0
	}

	return dagst, w, nil
}

// newDatastore creates a datastore under the given base directory
// for dagstore metadata.
func newDatastore(dir string) (ds.Batching, error) {
	// Create the datastore directory if it doesn't exist yet.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s for DAG store datastore: %w", dir, err)
	}

	// Create a new LevelDB datastore
	dstore, err := levelds.NewDatastore(dir, &levelds.Options{
		Compression: ldbopts.NoCompression,
		NoSync:      false,
		Strict:      ldbopts.StrictAll,
		ReadOnly:    false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open datastore for DAG store: %w", err)
	}
	// Keep statistics about the datastore
	mds := measure.New("measure.", dstore)
	return mds, nil
}

func (w *Wrapper) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	// Run a go-routine to do DagStore GC.
	w.backgroundWg.Add(1)
	go w.gcLoop()

	// Run a go-routine for shard recovery
	if dss, ok := w.dagst.(*dagstore.DAGStore); ok {
		w.backgroundWg.Add(1)
		go dagstore.RecoverImmediately(w.ctx, dss, w.failureCh, maxRecoverAttempts, w.backgroundWg.Done)
	}

	now := time.Now()
	err := w.dagst.Start(ctx)
	if err != nil {
		log.Errorf("failed to start dagstore: %s", err)
	}
	log.Debugf("dagstore started in %s, err: %v", time.Since(now), err)

	return nil
}

func (w *Wrapper) gcLoop() {
	defer w.backgroundWg.Done()

	if w.gcInterval == 0 {
		return
	}

	ticker := time.NewTicker(w.gcInterval)
	defer ticker.Stop()

	for w.ctx.Err() == nil {
		select {
		// GC the DAG store on every tick
		case <-ticker.C:
			_, _ = w.dagst.GC(w.ctx)

		// Exit when the DAG store wrapper is shutdown
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *Wrapper) LoadShard(ctx context.Context, pieceCid cid.Cid) (stores.ClosableBlockstore, error) {
	log := log.With("piece-cid", pieceCid)
	log.Debug("acquiring shard")

	key := shard.KeyFromCID(pieceCid)

	// get shard info
	var sInfo dagstore.ShardInfo
	var err error
	retryCount := 5
	for i := retryCount; i >= 0; i-- {
		if i == 0 {
			return nil, fmt.Errorf("failed to get shard info for piece CID  %s, after %d retry : %w", pieceCid, i, err)
		}

		sInfo, err = w.dagst.GetShardInfo(key)
		if err != nil {
			if errors.Is(err, dagstore.ErrShardUnknown) {
				log.Warn("shard not found, try to re-register")
				if err := stores.RegisterShardSync(ctx, w, pieceCid, "", false); err != nil {
					return nil, fmt.Errorf("failed to re-register shard during loading pieceCID %s: %w", pieceCid, err)
				}
				continue
			} else {
				return nil, fmt.Errorf("failed to get shard info for piece CID %s: %w", pieceCid, err)
			}
		}
		break
	}

	// check state
	log.Infof("shard state: %s", sInfo.ShardState.String())
	switch sInfo.ShardState {
	case dagstore.ShardStateErrored:
		// try to recover
		log.Warn("shard is in errored state, try to recover")
		recoverRes := make(chan dagstore.ShardResult, 1)
		if err := w.dagst.RecoverShard(ctx, key, recoverRes, dagstore.RecoverOpts{}); err != nil {
			return nil, fmt.Errorf("failed to recover shard for piece CID %s: %w", pieceCid, err)
		}
		select {
		case res := <-recoverRes:
			if res.Error != nil {
				return nil, fmt.Errorf("failed to recover shard for piece CID %s: %w", pieceCid, res.Error)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	resCh := make(chan dagstore.ShardResult, 1)
	err = w.dagst.AcquireShard(ctx, key, resCh, dagstore.AcquireOpts{})
	log.Debugf("sent message to acquire shard for piece CID %s", pieceCid)

	if err != nil {
		return nil, fmt.Errorf("failed to acquire shard for piece CID %s: %w", pieceCid, err)
	}

	// TODO: The context is not yet being actively monitored by the DAG store,
	// so we need to select against ctx.Done() until the following issue is
	// implemented:
	// https://github.com/filecoin-project/dagstore/issues/39
	var res dagstore.ShardResult
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res = <-resCh:
		if res.Error != nil {
			return nil, fmt.Errorf("failed to acquire shard for piece CID %s: %w", pieceCid, res.Error)
		}
	}

	bs, err := res.Accessor.Blockstore()
	if err != nil {
		return nil, err
	}

	log.Debugf("successfully loaded blockstore for piece CID %s", pieceCid)
	return &Blockstore{ReadBlockstore: bs, Closer: res.Accessor}, nil
}

func (w *Wrapper) RegisterShard(ctx context.Context, pieceCid cid.Cid, carPath string, eagerInit bool, resch chan dagstore.ShardResult) error {
	// Create a lotus mount with the piece CID
	key := shard.KeyFromCID(pieceCid)
	mt, err := NewPieceMount(pieceCid, w.cfg.UseTransient, w.minerAPI)
	if err != nil {
		return fmt.Errorf("failed to create lotus mount for piece CID %s: %w", pieceCid, err)
	}

	if resch == nil {
		sInfo, err := w.dagst.GetShardInfo(key)

		if err == nil && (sInfo.ShardState == dagstore.ShardStateAvailable ||
			sInfo.ShardState == dagstore.ShardStateServing || sInfo.ShardState == dagstore.ShardStateNew) {
			return nil
		}
	}

	// Register the shard
	opts := dagstore.RegisterOpts{
		ExistingTransient:  carPath,
		LazyInitialization: !eagerInit,
	}
	err = w.dagst.RegisterShard(ctx, key, mt, resch, opts)
	if err != nil {
		return fmt.Errorf("failed to schedule register shard for piece CID %s: %w", pieceCid, err)
	}
	log.Debugf("successfully submitted Register Shard request for piece CID %s with eagerInit=%t", pieceCid, eagerInit)

	return nil
}

func (w *Wrapper) DestroyShard(ctx context.Context, pieceCid cid.Cid, resch chan dagstore.ShardResult) error {
	key := shard.KeyFromCID(pieceCid)

	opts := dagstore.DestroyOpts{}

	err := w.dagst.DestroyShard(ctx, key, resch, opts)
	if err != nil {
		return fmt.Errorf("failed to schedule destroy shard for piece CID %s: %w", pieceCid, err)
	}
	log.Debugf("successfully submitted destroy Shard request for piece CID %s", pieceCid)

	return nil
}

func (w *Wrapper) MigrateDeals(ctx context.Context, deals []storagemarket.MinerDeal) (bool, error) {
	log := log.Named("migrator")

	// Check if all deals have already been registered as shards
	isComplete, err := w.registrationComplete()
	if err != nil {
		return false, fmt.Errorf("failed to get dagstore migration status: %w", err)
	}
	if isComplete {
		// All deals have been registered as shards, bail out
		log.Info("no shard migration necessary; already marked complete")
		return false, nil
	}

	log.Infow("registering shards for all active deals in sealing subsystem", "count", len(deals))

	inSealingSubsystem := make(map[fsm.StateKey]struct{}, len(providerstates.StatesKnownBySealingSubsystem))
	for _, s := range providerstates.StatesKnownBySealingSubsystem {
		inSealingSubsystem[s] = struct{}{}
	}

	// channel where results will be received, and channel where the total
	// number of registered shards will be sent.
	resch := make(chan dagstore.ShardResult, 32)
	totalCh := make(chan int)
	doneCh := make(chan struct{})

	// Start making progress consuming results. We won't know how many to
	// actually consume until we register all shards.
	//
	// If there are any problems registering shards, just log an error
	go func() {
		defer close(doneCh)

		total := math.MaxInt64
		var res dagstore.ShardResult
		for rcvd := 0; rcvd < total; {
			select {
			case total = <-totalCh:
				// we now know the total number of registered shards
				// nullify so that we no longer consume from it after closed.
				close(totalCh)
				totalCh = nil
			case res = <-resch:
				rcvd++
				if res.Error == nil {
					log.Infow("async shard registration completed successfully", "shard_key", res.Key)
				} else {
					log.Warnw("async shard registration failed", "shard_key", res.Key, "error", res.Error)
				}
			}
		}
	}()

	// Filter for deals that are handed off.
	//
	// If the deal has not yet been handed off to the sealing subsystem, we
	// don't need to call RegisterShard in this migration; RegisterShard will
	// be called in the new code once the deal reaches the state where it's
	// handed off to the sealing subsystem.
	var registered int
	for _, deal := range deals {
		pieceCid := deal.Proposal.PieceCID

		// enrich log statements in this iteration with deal ID and piece CID.
		log := log.With("deal_id", deal.DealID, "piece_cid", pieceCid)

		// Filter for deals that have been handed off to the sealing subsystem
		if _, ok := inSealingSubsystem[deal.State]; !ok {
			log.Infow("deal not ready; skipping")
			continue
		}

		log.Infow("registering deal in dagstore with lazy init")

		// Register the deal as a shard with the DAG store with lazy initialization.
		// The index will be populated the first time the deal is retrieved, or
		// through the bulk initialization script.
		err = w.RegisterShard(ctx, pieceCid, "", false, resch)
		if err != nil {
			log.Warnw("failed to register shard", "error", err)
			continue
		}
		registered++
	}

	log.Infow("finished registering all shards", "total", registered)
	totalCh <- registered
	<-doneCh

	log.Infow("confirmed registration of all shards")

	// Completed registering all shards, so mark the migration as complete
	err = w.markRegistrationComplete()
	if err != nil {
		log.Errorf("failed to mark shards as registered: %s", err)
	} else {
		log.Info("successfully marked migration as complete")
	}

	log.Infow("dagstore migration complete")

	return true, nil
}

// Check for the existence of a "marker" file indicating that the migration
// has completed
func (w *Wrapper) registrationComplete() (bool, error) {
	path := filepath.Join(w.cfg.RootDir, shardRegMarker)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Create a "marker" file indicating that the migration has completed
func (w *Wrapper) markRegistrationComplete() error {
	path := filepath.Join(w.cfg.RootDir, shardRegMarker)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

// Get all the pieces that contain a block
func (w *Wrapper) GetPiecesContainingBlock(blockCID cid.Cid) ([]cid.Cid, error) {
	// Pieces are stored as "shards" in the DAG store
	shardKeys, err := w.dagst.ShardsContainingMultihash(w.ctx, blockCID.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting pieces containing block %s: %w", blockCID, err)
	}

	// Convert from shard key to cid
	pieceCids := make([]cid.Cid, 0, len(shardKeys))
	for _, k := range shardKeys {
		c, err := cid.Parse(k.String())
		if err != nil {
			prefix := fmt.Sprintf("getting pieces containing block %s:", blockCID)
			return nil, fmt.Errorf("%s converting shard key %s to piece cid: %w", prefix, k, err)
		}

		pieceCids = append(pieceCids, c)
	}

	return pieceCids, nil
}

func (w *Wrapper) GetIterableIndexForPiece(pieceCid cid.Cid) (carindex.IterableIndex, error) {
	return w.dagst.GetIterableIndex(shard.KeyFromCID(pieceCid))
}

func (w *Wrapper) Close() error {
	// Cancel the context
	w.cancel()

	// Close the DAG store
	log.Info("will close the dagstore")
	if err := w.dagst.Close(); err != nil {
		return fmt.Errorf("failed to close dagstore: %w", err)
	}
	log.Info("dagstore closed")

	// Wait for the background go routine to exit
	log.Info("waiting for dagstore background wrapper goroutines to exit")
	w.backgroundWg.Wait()
	log.Info("exited dagstore background wrapper goroutines")

	return nil
}
