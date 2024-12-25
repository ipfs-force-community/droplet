package storageprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs-force-community/droplet/v2/indexprovider"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs-force-community/droplet/v2/utils"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
)

var directDealLog = logging.Logger("direct_deal_provider")

type DirectDealProvider struct {
	dealRepo         repo.DirectDealRepo
	pieceStorageMgr  *piecestorage.PieceStorageManager
	spn              StorageProviderNode
	fullNode         v1.FullNode
	dagStoreWrapper  stores.DAGStoreWrapper
	indexProviderMgr *indexprovider.IndexProviderMgr
}

func NewDirectDealProvider(lc fx.Lifecycle,
	spn StorageProviderNode,
	repo repo.Repo,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	fullNode v1.FullNode,
	dagStoreWrapper stores.DAGStoreWrapper,
	indexProviderMgr *indexprovider.IndexProviderMgr,
) (*DirectDealProvider, error) {
	ddp := &DirectDealProvider{
		spn:              spn,
		dealRepo:         repo.DirectDealRepo(),
		pieceStorageMgr:  pieceStorageMgr,
		fullNode:         fullNode,
		dagStoreWrapper:  dagStoreWrapper,
		indexProviderMgr: indexProviderMgr,
	}

	t := newTracker(repo.DirectDealRepo(), fullNode, indexProviderMgr)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go t.start(ctx)
			return nil
		},
	})

	return ddp, nil
}

type commonParams struct {
	filePath          string
	skipCommP         bool
	noCopyCarFile     bool
	skipGenerateIndex bool
}

func (ddp *DirectDealProvider) ImportDeals(ctx context.Context, dealParams *types.DirectDealParams) error {
	cParams := &commonParams{
		skipCommP:         dealParams.SkipCommP,
		skipGenerateIndex: dealParams.SkipGenerateIndex,
		noCopyCarFile:     dealParams.NoCopyCarFile,
	}
	errs := &multierror.Error{}
	for idx, dealParam := range dealParams.DealParams {
		cParams.filePath = dealParam.FilePath
		if err := ddp.importDeal(ctx, &dealParams.DealParams[idx], cParams); err != nil {
			errs = multierror.Append(fmt.Errorf("import deal failed, allocation id: %d, error: %v",
				dealParam.AllocationID, err), errs)
		}
	}

	return errs.ErrorOrNil()
}

func (ddp *DirectDealProvider) importDeal(ctx context.Context, dealParam *types.DirectDealParam, cParams *commonParams) error {
	deal, err := ddp.dealRepo.GetDealByAllocationID(ctx, dealParam.AllocationID)
	if err == nil {
		return fmt.Errorf("deal(%v) exist: %s", deal.AllocationID, deal.State.String())
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return err
	}
	// deal not exist
	deal = &types.DirectDeal{
		ID:           uuid.New(),
		PieceCID:     dealParam.PieceCID,
		Client:       dealParam.Client,
		State:        types.DealAllocated,
		AllocationID: dealParam.AllocationID,
		PayloadSize:  dealParam.PayloadSize,
		StartEpoch:   dealParam.StartEpoch,
		EndEpoch:     dealParam.EndEpoch,
	}
	if err := ddp.accept(ctx, deal); err != nil {
		return err
	}

	if err := ddp.importData(ctx, deal, cParams); err != nil {
		return err
	}

	if deal.PayloadSize == 0 {
		return fmt.Errorf("payload size is 0")
	}
	directDealLog.Infof("deal piece cid: %s, allocation id: %d, payload size: %d", deal.PieceCID, deal.AllocationID, deal.PayloadSize)

	if err := ddp.dealRepo.SaveDeal(ctx, deal); err != nil {
		return err
	}

	if !cParams.noCopyCarFile && !cParams.skipCommP {
		directDealLog.Infof("register shard. deal:%v, allocationID:%d, pieceCid:%s", deal.ID, deal.AllocationID, deal.PieceCID)
		// Register the deal data as a "shard" with the DAG store. Later it can be
		// fetched from the DAG store during retrieval.
		if err := ddp.dagStoreWrapper.RegisterShard(ctx, deal.PieceCID, "", true, nil); err != nil {
			directDealLog.Errorf("failed to register shard: %v", err)
		}
	}

	return nil
}

func (ddp *DirectDealProvider) accept(ctx context.Context, deal *types.DirectDeal) error {
	chainHead, err := ddp.spn.ChainHead(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain head: %v", err)
	}
	allocation, err := ddp.fullNode.StateGetAllocation(ctx, deal.Client, verifreg.AllocationId(deal.AllocationID), shared.EmptyTSK)
	if err != nil {
		return fmt.Errorf("failed to get allocation(%d): %w", deal.AllocationID, err)
	}
	if allocation == nil {
		return fmt.Errorf("allocation %d not found for client %s", deal.AllocationID, deal.Client)
	}

	if chainHead.Height() > allocation.Expiration {
		return fmt.Errorf(
			"cannot propose direct deal with piece CID %s: current epoch %d has passed direct deal proposal start epoch %d",
			deal.PieceCID, chainHead.Height(), allocation.Expiration)
	}

	deal.PieceSize = allocation.Size
	deal.Provider, err = address.NewIDAddress(uint64(allocation.Provider))
	if err != nil {
		return fmt.Errorf("parse %d to address failed: %v", allocation.Provider, err)
	}

	directDealLog.Infow("found allocation for client", "allocation", spew.Sdump(*allocation))

	return nil
}

func (ddp *DirectDealProvider) importData(ctx context.Context, deal *types.DirectDeal, cParams *commonParams) error {
	// not copy file to piece storage and not verify commp and skip generate index
	if cParams.noCopyCarFile {
		return nil
	}

	var r io.ReadCloser
	var carSize int64
	var pieceStore piecestorage.IPieceStorage
	var err error
	pieceCIDStr := deal.PieceCID.String()

	getReader := func() (io.ReadCloser, error) {
		pieceStore, err = ddp.pieceStorageMgr.FindStorageForRead(ctx, pieceCIDStr)
		if err == nil {
			directDealLog.Debugf("found %v already in piece storage", pieceCIDStr)

			carSize, err = pieceStore.Len(ctx, pieceCIDStr)
			if err != nil {
				return nil, fmt.Errorf("got piece size from piece store failed: %v", err)
			}
			readerCloser, err := pieceStore.GetReaderCloser(ctx, pieceCIDStr)
			if err != nil {
				return nil, fmt.Errorf("got reader from piece store failed: %v", err)
			}
			return readerCloser, nil
		}
		directDealLog.Debugf("not found %s in piece storage", pieceCIDStr)

		info, err := os.Stat(cParams.filePath)
		if err != nil {
			return nil, err
		}
		carSize = info.Size()

		return os.Open(cParams.filePath)
	}

	r, err = getReader()
	if err != nil {
		return err
	}
	deal.PayloadSize = uint64(carSize)

	defer func() {
		if err = r.Close(); err != nil {
			log.Errorf("unable to close reader: %v, %v", pieceCIDStr, err)
		}
	}()

	if !cParams.skipCommP {
		proofType, err := ddp.spn.GetProofType(ctx, deal.Provider, nil) // TODO: 判断是不是属于此矿池?
		if err != nil {
			return fmt.Errorf("failed to determine proof type: %w", err)
		}

		pieceCid, err := utils.GeneratePieceCommP(proofType, r, uint64(carSize), uint64(deal.PieceSize))
		if err != nil {
			return fmt.Errorf("generate commp failed: %v", err)
		}

		if !pieceCid.Equals(deal.PieceCID) {
			return fmt.Errorf("given data does not match expected commP (got: %s, expected %s)", pieceCid, deal.PieceCID)
		}

		if err := r.Close(); err != nil {
			log.Errorf("unable to close reader: %v, %v", pieceCIDStr, err)
		}

		r, err = getReader()
		if err != nil {
			return err
		}
	}

	// copy car file to piece storage
	if pieceStore == nil {
		pieceStore, err = ddp.pieceStorageMgr.FindStorageForWrite(carSize)
		if err != nil {
			return err
		}
		if _, err := pieceStore.SaveTo(ctx, pieceCIDStr, r); err != nil {
			return fmt.Errorf("copy car file to piece storage failed: %v", err)
		}
	}

	return nil
}

type tracker struct {
	directDealRepo   repo.DirectDealRepo
	fullNode         v1.FullNode
	indexProviderMgr *indexprovider.IndexProviderMgr
}

func newTracker(directDealRepo repo.DirectDealRepo,
	fullNode v1.FullNode,
	indexProviderMgr *indexprovider.IndexProviderMgr,
) *tracker {
	return &tracker{
		directDealRepo:   directDealRepo,
		fullNode:         fullNode,
		indexProviderMgr: indexProviderMgr,
	}
}

func (t *tracker) start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute*15 + time.Minute*time.Duration(globalRand.Intn(15)))
	defer ticker.Stop()

	slashTicker := time.NewTicker(time.Hour*2 + time.Minute*time.Duration(globalRand.Intn(60)))
	defer slashTicker.Stop()

	if err := t.trackDeals(ctx); err != nil {
		directDealLog.Warnf("track deals failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-slashTicker.C:
			if err := t.checkSlash(ctx); err != nil {
				directDealLog.Warnf("check slash failed: %v", err)
			}
		case <-ticker.C:
			if err := t.trackDeals(ctx); err != nil {
				directDealLog.Warnf("track deals failed: %v", err)
			}
		}
	}
}

func (t *tracker) trackDeals(ctx context.Context) error {
	head, err := t.fullNode.ChainHead(ctx)
	if err != nil {
		return err
	}

	dealAllocation := types.DealAllocated
	deals, err := t.directDealRepo.ListDeal(ctx, types.DirectDealQueryParams{
		State: &dealAllocation,
		Page: types.Page{
			Limit: math.MaxInt64,
		},
	})
	if err != nil {
		return err
	}
	for _, deal := range deals {
		if head.Height() > deal.StartEpoch {
			deal.State = types.DealExpired
			if err := t.directDealRepo.SaveDeal(ctx, deal); err != nil {
				return err
			}
		}
	}

	if err := t.checkActive(ctx); err != nil {
		return err
	}

	return nil
}

func (t *tracker) checkActive(ctx context.Context) error {
	dealSealing := types.DealSealing
	deals, err := t.directDealRepo.ListDeal(ctx, types.DirectDealQueryParams{
		State: &dealSealing,
		Page: types.Page{
			Limit: math.MaxInt64,
		},
	})
	if err != nil {
		return err
	}

	for _, d := range deals {
		// allocation id and claim id are the same
		claim, err := t.fullNode.StateGetClaim(ctx, d.Provider, verifreg.ClaimId(d.AllocationID), shared.EmptyTSK)
		if err != nil {
			directDealLog.Debugf("get claim %d by allocation id %d failed: %v", d.AllocationID, err)
			continue
		}
		if claim == nil {
			continue
		}

		d.State = types.DealActive
		if err := t.directDealRepo.SaveDeal(ctx, d); err != nil {
			return err
		}
	}

	return nil
}

func (t *tracker) checkSlash(ctx context.Context) error {
	head, err := t.fullNode.ChainHead(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute*110)
	defer cancel()

	// todo: use AllocationID to find deal?
	offset := 0
	limit := 1000
	dealActive := types.DealActive
	for ctx.Err() == nil {
		deals, err := t.directDealRepo.ListDeal(ctx, types.DirectDealQueryParams{
			State: &dealActive,
			Page: types.Page{
				Limit:  limit,
				Offset: offset,
			},
		})
		if err != nil {
			return err
		}

		for _, deal := range deals {
			claim, err := t.fullNode.StateGetClaim(ctx, deal.Provider, verifreg.ClaimId(deal.AllocationID), shared.EmptyTSK)
			if err != nil {
				directDealLog.Debugf("get claim %d failed: %v", deal.AllocationID, err)
				continue
			}
			if claim == nil {
				continue
			}

			if head.Height() >= claim.TermStart+claim.TermMax {
				deal.State = types.DealSlashed
				if err := t.directDealRepo.SaveDeal(ctx, deal); err != nil {
					return err
				}
				contextID, err := deal.ID.MarshalBinary()
				if err != nil {
					return fmt.Errorf("deal %s marshal binary failed: %v", deal.ID, err)
				}
				_, err = t.indexProviderMgr.AnnounceDealRemoved(ctx, deal.Provider, contextID)
				if err != nil {
					return fmt.Errorf("announce deal %s removed failed: %v", deal.ID, err)
				}
			}
		}

		if len(deals) < limit {
			break
		}
		offset += len(deals)
	}

	return nil
}
