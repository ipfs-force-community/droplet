package storageprovider

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/transport/httptransport"
	types2 "github.com/filecoin-project/venus-market/v2/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

type DealTransport struct {
	ctx context.Context

	dealStore   repo.StorageDealRepo
	dealProcess StorageDealHandler
	spn         StorageProviderNode
	fs          filestore.FileStore

	httpTransport httptransport.Transport
	handlers      map[cid.Cid]httptransport.Handler

	lk sync.Mutex
}

func NewDealTransport(mctx metrics.MetricsCtx,
	lc fx.Lifecycle,
	cfg *config.MarketConfig,
	repo repo.Repo,
	dealProcess StorageDealHandler,
	spn StorageProviderNode,
	fs filestore.FileStore,
	host host.Host) (*DealTransport, error) {
	opt := httptransport.BackOffRetryOpt(time.Duration(cfg.TransportConfig.MinBackOffWait), time.Duration(cfg.TransportConfig.MaxBackoffWait),
		cfg.TransportConfig.BackOffFactor, cfg.TransportConfig.MaxReconnectAttempts)
	httpTransport := httptransport.New(host, opt)

	dt := &DealTransport{
		ctx:           mctx,
		dealStore:     repo.StorageDealRepo(),
		dealProcess:   dealProcess,
		spn:           spn,
		fs:            fs,
		httpTransport: httpTransport,
		handlers:      make(map[cid.Cid]httptransport.Handler),
		lk:            sync.Mutex{},
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return dt.restartTransport(ctx)
		},
		OnStop: func(ctx context.Context) error {
			dt.Close()
			return nil
		},
	})

	return dt, nil
}

func isValidDeal(deal *types.MinerDeal) error {
	if deal.State == storagemarket.StorageDealWaitingForData {
		return nil
	}
	if isTerminateState(deal) {
		return xerrors.Errorf("deal %s is terminate state", deal.ProposalCid)
	}
	if deal.State > storagemarket.StorageDealWaitingForData {
		return xerrors.Errorf("deal %s does not support offline data", deal.ProposalCid)
	}

	return xerrors.Errorf("deal %s state %d is invalid", deal.ProposalCid, deal.State)
}

func (dt *DealTransport) TransportData(ctx context.Context, ti *types2.TransportInfo, deal *types.MinerDeal) error {
	if err := isValidDeal(deal); err != nil {
		return err
	}

	// setup clean-up code
	var tmpFile filestore.File
	cleanup := func() {
		if tmpFile != nil {
			_ = os.Remove(string(tmpFile.OsPath()))
		}
	}

	// create a temp file where we will hold the deal data.
	tmpFile, err := dt.fs.CreateTemp()
	if err != nil {
		cleanup()
		return xerrors.Errorf("failed to create temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return xerrors.Errorf("failed to close temp file: %w", err)
	}
	ti.OutputFile = string(tmpFile.OsPath())
	log.Infof("deal %v output file %s", ti.ProposalCID, ti.OutputFile)

	deal.Ref.State = int64(types2.Transporting)
	deal.PiecePath = tmpFile.Path()
	if err := dt.dealStore.SaveDeal(dt.ctx, deal); err != nil {
		return xerrors.Errorf("save deal %s failed: %v", ti.ProposalCID, err)
	}

	return dt.startTransport(dt.ctx, ti, deal)
}

func (dt *DealTransport) verifyPieceCid(ctx context.Context, filePath string, d *types.MinerDeal) error {
	_, fileName := filepath.Split(filePath)
	tempfi, err := dt.fs.Open(filestore.Path(fileName))
	if err != nil {
		return xerrors.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := tempfi.Close(); err != nil {
			log.Errorf("unable to close stream %v", err)
		}
	}()
	cleanup := func() {
		_ = tempfi.Close()
		_ = dt.fs.Delete(tempfi.Path())
	}

	if err := verifyPieceCid(ctx, dt.spn, d, tempfi); err != nil {
		cleanup()
		return err
	}
	return nil
}

func (dt *DealTransport) transportCompleted(ctx context.Context, deal *types.MinerDeal) error {
	deal.MetadataPath = ""
	deal.Ref.State = int64(types2.TransportCompleted)
	log.Infof("deal %s piece path: %s", deal.ProposalCid, deal.PiecePath)

	deal.State = storagemarket.StorageDealReserveProviderFunds
	deal.PieceStatus = types.Undefine
	if err := dt.dealStore.SaveDeal(ctx, deal); err != nil {
		return err
	}
	go func() {
		err := dt.dealProcess.HandleOff(context.TODO(), deal)
		if err != nil {
			log.Errorf("deal %s handle off err: %s", deal.ProposalCid, err)
		}
	}()

	return nil
}

func (dt *DealTransport) startTransport(ctx context.Context, ti *types2.TransportInfo, deal *types.MinerDeal) error {
	dt.lk.Lock()
	// build in-memory state
	fi, err := os.Stat(ti.OutputFile)
	if err != nil {
		dt.lk.Unlock()
		return xerrors.Errorf("failed to stat output file: %w", err)
	}
	ti.NBytesReceived = fi.Size()

	st := time.Now()
	handler, err := dt.httpTransport.Execute(ctx, ti)
	if err != nil {
		dt.lk.Unlock()
		return err
	}
	dt.handlers[ti.ProposalCID] = handler
	dt.lk.Unlock()

	defer dt.removeHandler(ti.ProposalCID)

	// wait for data-transfer to finish
	if err := dt.waitForTransferFinish(ctx, handler, ti); err != nil {
		deal.Ref.State = int64(types2.TransportFailed)
		if err2 := dt.dealStore.SaveDeal(ctx, deal); err2 != nil {
			log.Errorf("save deal %s failed: %v", ti.ProposalCID, err2)
		}
		return xerrors.Errorf("data-transfer failed: %w", err)
	}

	log.Infof("deal %s data-transfer completed successfully, bytes received %d, time taken %v", ti.ProposalCID,
		ti.NBytesReceived, time.Since(st).String())

	if err := dt.verifyPieceCid(ctx, ti.OutputFile, deal); err != nil {
		deal.Ref.State = int64(types2.TransportFailed)
		if err2 := dt.dealStore.SaveDeal(ctx, deal); err2 != nil {
			log.Errorf("save deal %s failed: %v", ti.ProposalCID, err2)
		}
		return err
	}
	log.Infof("deal %s commP matched successfully: deal-data verified", ti.ProposalCID)

	return dt.transportCompleted(ctx, deal)
}

func (dt *DealTransport) waitForTransferFinish(ctx context.Context, handler httptransport.Handler, info *types2.TransportInfo) error {
	defer handler.Close()
	var lastOutputPct int64

	logTransferProgress := func(received int64) {
		pct := (100 * received) / int64(info.Transfer.Size)
		outputPct := pct / 10
		if outputPct != lastOutputPct {
			lastOutputPct = outputPct
			log.Infof("deal %s transfer progress, bytes received %v, deal size %v, percent complete %v", info.ProposalCID, received, info.Transfer.Size, pct)
		}
	}

	for {
		select {
		case evt, ok := <-handler.Sub():
			if !ok {
				return nil
			}
			if evt.Error != nil {
				return evt.Error
			}
			info.NBytesReceived = evt.NBytesReceived
			logTransferProgress(info.NBytesReceived)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (dt *DealTransport) Exist(propCid cid.Cid) bool {
	dt.lk.Lock()
	defer dt.lk.Unlock()
	_, ok := dt.handlers[propCid]

	return ok
}

func (dt *DealTransport) removeHandler(propCid cid.Cid) {
	dt.lk.Lock()
	defer dt.lk.Unlock()

	delete(dt.handlers, propCid)
}

func (dt *DealTransport) restartTransport(ctx context.Context) error {
	deals, err := dt.dealStore.ListTransportUnCompleteDeal(ctx)
	if err != nil {
		return err
	}
	for _, deal := range deals {
		if deal.Ref.State == int64(types2.Transporting) {
			continue
		}
		f, err := dt.fs.Open(deal.PiecePath)
		if err != nil {
			log.Warnf("deal %v open file [PiecePath: %s] failed %v", deal.ProposalCid, deal.PiecePath, err)
			continue
		}
		info := &types2.TransportInfo{
			ProposalCID: deal.ProposalCid,
			OutputFile:  string(f.OsPath()),
			Transfer: types.Transfer{
				Type:   deal.Ref.TransferType,
				Params: deal.Ref.Params,
				Size:   deal.Ref.RawBlockSize,
			},
			NBytesReceived: f.Size(),
		}
		_ = f.Close()

		go func(info *types2.TransportInfo, deal *types.MinerDeal) {
			if err := isValidDeal(deal); err != nil {
				log.Info(err)
				return
			}
			log.Infof("start transport data %v", deal.ProposalCid)
			if err := dt.startTransport(ctx, info, deal); err != nil {
				log.Errorf("deal %v transport data failed %v", info, err)
			}
		}(info, deal)
	}

	return nil
}

func (dt *DealTransport) Close() {
	dt.lk.Lock()
	defer dt.lk.Unlock()

	for _, h := range dt.handlers {
		h.Close()
	}
}
