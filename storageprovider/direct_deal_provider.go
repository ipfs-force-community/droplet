package storageprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/filecoin-project/go-address"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs-force-community/droplet/v2/utils"
)

type DirectDealProvider struct {
	dealRepo        repo.DirectDealRepo
	pieceStorageMgr *piecestorage.PieceStorageManager
	spn             StorageProviderNode
}

func NewDirectDealProvider(spn StorageProviderNode,
	repo repo.Repo,
	pieceStorageMgr *piecestorage.PieceStorageManager,
) (*DirectDealProvider, error) {
	ddp := &DirectDealProvider{
		spn:             spn,
		dealRepo:        repo.DirectDealRepo(),
		pieceStorageMgr: pieceStorageMgr,
	}

	return ddp, nil
}

func (ddp *DirectDealProvider) ImportDeal(ctx context.Context, dealParams *types.DirectDealParams) error {
	deal, err := ddp.dealRepo.GetDealByAllocationID(ctx, uint64(dealParams.AllocationID))
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			return err
		}
		// deal not exist
		deal = &types.DirectDeal{
			ID:           uuid.New(),
			PieceCID:     dealParams.PieceCID,
			Client:       dealParams.Client,
			State:        types.DealAllocation,
			PieceStatus:  types.Undefine,
			AllocationID: dealParams.AllocationID,
			StartEpoch:   dealParams.StartEpoch,
			EndEpoch:     dealParams.EndEpoch,
		}
		if err := ddp.accept(ctx, deal); err != nil {
			ddp.errorDeal(ctx, deal, err.Error())
			return err
		}
		deal.State = types.DealWaitForData
		if err := ddp.dealRepo.SaveDeal(ctx, deal); err != nil {
			return err
		}
	} else {
		if deal.State > types.DealWaitForData {
			return fmt.Errorf("deal exist: %s", deal.State.String())
		}
	}
	if err = ddp.importData(ctx, deal, dealParams); err != nil {
		return fmt.Errorf("import deal data failed: %v", err)
	}

	deal.State = types.DealAwaitingPreCommit
	if err := ddp.dealRepo.SaveDeal(ctx, deal); err != nil {
		return err
	}

	return nil
}

func (ddp *DirectDealProvider) accept(ctx context.Context, deal *types.DirectDeal) error {
	chainHead, err := ddp.spn.ChainHead(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain head: %v", err)
	}

	// if chainHead.Height()+ddp.config.StartEpochSealingBuffer > entry.StartEpoch {
	if chainHead.Height() > deal.StartEpoch {
		return fmt.Errorf(
			"cannot propose direct deal with piece CID %s: current epoch %d has passed direct deal proposal start epoch %d",
			deal.PieceCID, chainHead.Height(), deal.StartEpoch)
	}

	allocation, err := ddp.spn.StateGetAllocation(ctx, deal.Client, deal.AllocationID, shared.EmptyTSK)
	if err != nil {
		return fmt.Errorf("failed to get allocations: %w", err)
	}
	if allocation == nil {
		return fmt.Errorf("allocation %d not found for client %s", deal.AllocationID, deal.Client)
	}
	deal.PieceSize = allocation.Size
	deal.Provider, err = address.NewIDAddress(uint64(allocation.Provider))
	if err != nil {
		return fmt.Errorf("parse %d to address failed: %v", allocation.Provider, err)
	}

	log.Infow("found allocation for client", "allocation", spew.Sdump(allocation))

	return nil
}

func (ddp *DirectDealProvider) importData(ctx context.Context, deal *types.DirectDeal, params *types.DirectDealParams) error {
	// not copy file to piece storage and not verify commp
	if params.NoCopyCarFile {
		return nil
	}

	var r io.ReadCloser
	var carSize int64

	pieceCIDStr := deal.PieceCID.String()
	pieceStore, err := ddp.pieceStorageMgr.FindStorageForRead(ctx, pieceCIDStr)
	if err == nil {
		log.Debugf("found %v already in piece storage", pieceCIDStr)

		carSize, err = pieceStore.Len(ctx, pieceCIDStr)
		if err != nil {
			return fmt.Errorf("got piece size from piece store failed: %v", err)
		}
		readerCloser, err := pieceStore.GetReaderCloser(ctx, pieceCIDStr)
		if err != nil {
			return fmt.Errorf("got reader from piece store failed: %v", err)
		}
		r = readerCloser
	} else {
		log.Debugf("not found %s in piece storage", pieceCIDStr)

		info, err := os.Stat(params.FilePath)
		if err != nil {
			return err
		}
		carSize = info.Size()

		r, err = os.Open(params.FilePath)
		if err != nil {
			return err
		}
	}

	defer func() {
		if err = r.Close(); err != nil {
			log.Errorf("unable to close reader: %v, %v", pieceCIDStr, err)
		}
	}()

	if !params.SkipCommP {
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

func (ddp *DirectDealProvider) errorDeal(ctx context.Context, deal *types.DirectDeal, message string) {
	deal.State = types.DealError
	deal.Message = message
	if err := ddp.dealRepo.SaveDeal(ctx, deal); err != nil {
		log.Errorf("save direct deal failed: %s, %v", deal.ID, err)
	}
}
