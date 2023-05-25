package retrievalprovider

import (
	"context"
	"errors"
	"fmt"
	"time"

	vtypes "github.com/filecoin-project/venus/venus-shared/types"
	gtypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
	mktypes "github.com/filecoin-project/venus/venus-shared/types/market"

	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statemachine"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	"github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
)

type IRetrievalHandler interface {
	UnsealData(ctx context.Context, deal *mktypes.ProviderDealState) error
	CancelDeal(ctx context.Context, deal *mktypes.ProviderDealState) error
	CleanupDeal(ctx context.Context, deal *mktypes.ProviderDealState) error
	Error(ctx context.Context, deal *mktypes.ProviderDealState, err error) error
}

var _ IRetrievalHandler = (*RetrievalDealHandler)(nil)

type RetrievalDealHandler struct {
	env                 ProviderDealEnvironment
	retrievalDealStore  repo.IRetrievalDealRepo
	storageDealRepo     repo.StorageDealRepo
	gatewayMarketClient gateway.IMarketClient
	pieceStorageMgr     *piecestorage.PieceStorageManager
}

func NewRetrievalDealHandler(env ProviderDealEnvironment, retrievalDealStore repo.IRetrievalDealRepo, storageDealRepo repo.StorageDealRepo, gatewayMarketClient gateway.IMarketClient, pieceStorageMgr *piecestorage.PieceStorageManager) IRetrievalHandler {
	return &RetrievalDealHandler{
		env:                 env,
		retrievalDealStore:  retrievalDealStore,
		storageDealRepo:     storageDealRepo,
		gatewayMarketClient: gatewayMarketClient,
		pieceStorageMgr:     pieceStorageMgr,
	}
}

func (p *RetrievalDealHandler) UnsealData(ctx context.Context, providerDeal *mktypes.ProviderDealState) (err error) {
	defer func() {
		if err != nil {
			log.Errorf("unseal data fail: %w", err)
		}
	}()

	log := log.With("dealId", providerDeal.ID)
	providerDeal.Status = rm.DealStatusUnsealing
	err = p.retrievalDealStore.SaveDeal(ctx, providerDeal)
	if err != nil {
		return
	}

	deal, err := p.storageDealRepo.GetDeal(ctx, providerDeal.SelStorageProposalCid)
	if err != nil {
		return
	}

	pieceCid := deal.Proposal.PieceCID
	log.With("pieceCid", pieceCid)

	// check piece exist

	st, err := p.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		// check fail, but unseal should continue
		log.Infof("try to find piece  fail: %w", err)
	}

	if st != nil {
		log.Info("piece already exist, no need to unseal")
	} else {
		// try unseal
		var wps piecestorage.IPieceStorage
		wps, err = p.pieceStorageMgr.FindStorageForWrite(int64(deal.Proposal.PieceSize))
		if err != nil {
			err = fmt.Errorf("failed to find storage to write %s: %w", deal.Proposal.PieceCID, err)
			return
		}

		var pieceTransfer string
		pieceTransfer, err = wps.GetPieceTransfer(ctx, pieceCid.String())
		if err != nil {
			err = fmt.Errorf("get piece transfer for %s: %w", pieceCid, err)
			return
		}

		log.Info("try to unseal")
		// should block util unseal finish or error, because it will resume transfer later
		state := gtypes.UnsealStateFailed
		checkUnsealInterval := 5 * time.Second
		tick := time.Tick(checkUnsealInterval)
		timeOutCtx, cancel := context.WithTimeout(ctx, 12*time.Hour)
		defer cancel()
		for state != gtypes.UnsealStateFinished {
			state, err = p.gatewayMarketClient.SectorsUnsealPiece(
				ctx,
				deal.Proposal.Provider,
				pieceCid,
				deal.SectorNumber,
				vtypes.UnpaddedByteIndex(deal.Offset.Unpadded()),
				deal.Proposal.PieceSize.Unpadded(),
				pieceTransfer,
			)
			if err != nil {
				err = fmt.Errorf("unseal piece %s: %w", pieceCid, err)
				return
			}
			if state == gtypes.UnsealStateFailed {
				err = fmt.Errorf("unseal piece %s fail: %w", pieceCid, err)
				return
			}
			log.Debugf("unseal piece %s: %s", pieceCid, state)
			select {
			case <-tick:
			case <-timeOutCtx.Done():
				err = ctx.Err()
				return
			}
		}
		log.Info("unseal piece success")
	}

	if err = p.env.PrepareBlockstore(ctx, providerDeal.ID, deal.Proposal.PieceCID); err != nil {
		log.Errorf("unable to load shard %s  %w", deal.Proposal.PieceCID, err)
		err = p.CancelDeal(ctx, providerDeal)
		return
	}
	log.Debugf("blockstore prepared successfully, firing unseal complete for deal %d", providerDeal.ID)
	providerDeal.Status = rm.DealStatusUnsealed
	err = p.retrievalDealStore.SaveDeal(ctx, providerDeal)
	if err != nil {
		return
	}

	log.Debugf("unpausing data transfer for deal %d", providerDeal.ID)

	if providerDeal.ChannelID != nil {
		log.Debugf("resuming data transfer for deal %d", providerDeal.ID)
		err = p.env.ResumeDataTransfer(ctx, *providerDeal.ChannelID)
		if err != nil {
			providerDeal.Status = rm.DealStatusErrored
		}
	}
	err = p.retrievalDealStore.SaveDeal(ctx, providerDeal)
	return
}

func (p *RetrievalDealHandler) CancelDeal(ctx context.Context, deal *mktypes.ProviderDealState) error {
	// Read next response (or fail)
	err := p.env.DeleteStore(deal.ID)
	if err != nil {
		return p.Error(ctx, deal, nil)
	}
	if deal.ChannelID != nil {
		err = p.env.CloseDataTransfer(ctx, *deal.ChannelID)
		if err != nil && !errors.Is(err, statemachine.ErrTerminated) {
			return p.Error(ctx, deal, nil)
		}
	}
	deal.Status = rm.DealStatusCancelled
	return p.retrievalDealStore.SaveDeal(ctx, deal)
}

// CleanupDeal runs to do memory cleanup for an in progress deal
func (p *RetrievalDealHandler) CleanupDeal(ctx context.Context, deal *mktypes.ProviderDealState) error {
	err := p.env.DeleteStore(deal.ID)
	if err != nil {
		return p.Error(ctx, deal, nil)
	}
	deal.Status = rm.DealStatusCompleted
	return p.retrievalDealStore.SaveDeal(ctx, deal)
}

func (p *RetrievalDealHandler) Error(ctx context.Context, deal *mktypes.ProviderDealState, err error) error {
	deal.Status = rm.DealStatusErrored
	if err != nil {
		deal.Message = err.Error()
	}
	return p.retrievalDealStore.SaveDeal(ctx, deal)
}
