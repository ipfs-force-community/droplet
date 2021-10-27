package retrievaladapter

import (
	"context"
	"errors"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/providerstates"
	"github.com/filecoin-project/go-statemachine"
)

type IRetrievalHandler interface {
}

var _ IRetrievalHandler = (*RetrievalDealHandler)(nil)

type RetrievalDealHandler struct {
	env                providerstates.ProviderDealEnvironment
	retrievalDealStore RetrievalDealStore
}

func NewRetrievalDealHandler(env providerstates.ProviderDealEnvironment, retrievalDealStore RetrievalDealStore) *RetrievalDealHandler {
	return &RetrievalDealHandler{env: env, retrievalDealStore: retrievalDealStore}
}

func (p *RetrievalDealHandler) UnsealData(ctx context.Context, deal *rm.ProviderDealState) error {
	if deal.Status == rm.DealStatusNew {
		deal.Status = rm.DealStatusUnsealing
		err := p.retrievalDealStore.SaveDeal(deal)
		if err != nil {
			return err
		}

		if err := p.env.PrepareBlockstore(ctx, deal.ID, deal.PieceInfo.PieceCID); err != nil {
			return p.CancelDeal(ctx, deal)
		}
		log.Debugf("blockstore prepared successfully, firing unseal complete for deal %d", deal.ID)
		deal.Status = rm.DealStatusUnsealed
		err = p.retrievalDealStore.SaveDeal(deal)
		if err != nil {
			return err
		}
	}

	log.Debugf("unpausing data transfer for deal %d", deal.ID)
	err := p.env.TrackTransfer(*deal)
	if err != nil {
		return p.CancelDeal(ctx, deal)
	}
	if deal.ChannelID != nil {
		log.Debugf("resuming data transfer for deal %d", deal.ID)
		err = p.env.ResumeDataTransfer(ctx, *deal.ChannelID)
		if err != nil {
			deal.Status = rm.DealStatusErrored
		}
	}
	return p.retrievalDealStore.SaveDeal(deal)
}

// TrackTransfer resumes a deal so we can start sending data after its unsealed
func (p *RetrievalDealHandler) TrackTransfer(ctx context.Context, deal *rm.ProviderDealState) error {
	err := p.env.TrackTransfer(*deal)
	if err != nil {
		deal.Status = rm.DealStatusErrored
	}
	return p.retrievalDealStore.SaveDeal(deal)
}

func (p *RetrievalDealHandler) CancelDeal(ctx context.Context, deal *rm.ProviderDealState) error {
	// Read next response (or fail)
	err := p.env.UntrackTransfer(*deal)
	if err != nil {
		deal.Status = rm.DealStatusErrored
		return p.retrievalDealStore.SaveDeal(deal)
	}
	err = p.env.DeleteStore(deal.ID)
	if err != nil {
		deal.Status = rm.DealStatusErrored
		return p.retrievalDealStore.SaveDeal(deal)
	}
	if deal.ChannelID != nil {
		err = p.env.CloseDataTransfer(ctx, *deal.ChannelID)
		if err != nil && !errors.Is(err, statemachine.ErrTerminated) {
			deal.Status = rm.DealStatusErrored
			return p.retrievalDealStore.SaveDeal(deal)
		}
	}
	deal.Status = rm.DealStatusCancelled
	return p.retrievalDealStore.SaveDeal(deal)
}

// CleanupDeal runs to do memory cleanup for an in progress deal
func (p *RetrievalDealHandler) CleanupDeal(ctx context.Context, deal *rm.ProviderDealState) error {
	err := p.env.UntrackTransfer(*deal)
	if err != nil {
		deal.Status = rm.DealStatusErrored
		return p.retrievalDealStore.SaveDeal(deal)
	}

	err = p.env.DeleteStore(deal.ID)
	if err != nil {
		deal.Status = rm.DealStatusErrored
		return p.retrievalDealStore.SaveDeal(deal)
	}
	deal.Status = rm.DealStatusCompleted
	return p.retrievalDealStore.SaveDeal(deal)
}

func (p *RetrievalDealHandler) Error(ctx context.Context, deal *rm.ProviderDealState, err error) error {
	deal.Status = rm.DealStatusErrored
	deal.Message = err.Error()
	return p.retrievalDealStore.SaveDeal(deal)
}
