package retrievaladapter

import (
	"context"

	"github.com/filecoin-project/venus-market/models/repo"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"
)

type IAskHandler interface {
	GetAsk(address.Address) (*retrievalmarket.Ask, error)
	SetAsk(address.Address, *retrievalmarket.Ask) error
	GetDynamicAsk(context.Context, retrievalmarket.PricingInput, []abi.DealID) (retrievalmarket.Ask, error)
	GetAskForPayload(context.Context, cid.Cid, *cid.Cid, piecestore.PieceInfo, bool, peer.ID) (retrievalmarket.Ask, error)
}

var _ IAskHandler = (*AskHandler)(nil)

type AskHandler struct {
	pieceStore           piecestore.PieceStore
	askStore             repo.IRetrievalAskRepo
	node                 retrievalmarket.RetrievalProviderNode
	retrievalPricingFunc retrievalimpl.RetrievalPricingFunc
}

func NewAskHandler(r repo.Repo, node retrievalmarket.RetrievalProviderNode, retrievalPricingFunc retrievalimpl.RetrievalPricingFunc) *AskHandler {
	return &AskHandler{askStore: r.RetrievalAskRepo(), node: node, retrievalPricingFunc: retrievalPricingFunc}
}

// GetAsk returns the current deal parameters this provider accepts
func (p *AskHandler) GetAsk(mAddr address.Address) (*retrievalmarket.Ask, error) {
	return p.askStore.GetAsk(mAddr)
}

// SetAsk sets the deal parameters this provider accepts
func (p *AskHandler) SetAsk(maddr address.Address, ask *retrievalmarket.Ask) error {
	return p.askStore.SetAsk(maddr, ask)
}

// GetDynamicAsk quotes a dynamic price for the retrieval deal by calling the user configured
// dynamic pricing function. It passes the static price parameters set in the Ask Store to the pricing function.
func (p *AskHandler) GetDynamicAsk(ctx context.Context, input retrievalmarket.PricingInput, storageDeals []abi.DealID) (retrievalmarket.Ask, error) {
	dp, err := p.node.GetRetrievalPricingInput(ctx, input.PieceCID, storageDeals)
	if err != nil {
		return retrievalmarket.Ask{}, xerrors.Errorf("GetRetrievalPricingInput: %s", err)
	}

	// currAsk cannot be nil as we initialize the ask store with a default ask.
	// Users can then change the values in the ask store using SetAsk but not remove it.
	currAsk, err := p.GetAsk(address.Undef) //todo use market payment address
	if currAsk == nil {
		return retrievalmarket.Ask{}, xerrors.New("no ask configured in ask-store")
	}

	dp.PayloadCID = input.PayloadCID
	dp.PieceCID = input.PieceCID
	dp.Unsealed = input.Unsealed
	dp.Client = input.Client
	dp.CurrentAsk = *currAsk

	ask, err := p.retrievalPricingFunc(ctx, dp)
	if err != nil {
		return retrievalmarket.Ask{}, xerrors.Errorf("retrievalPricingFunc: %w", err)
	}
	return ask, nil
}

func (p *AskHandler) GetAskForPayload(ctx context.Context, payloadCid cid.Cid, pieceCid *cid.Cid, piece piecestore.PieceInfo, isUnsealed bool, client peer.ID) (retrievalmarket.Ask, error) {

	storageDeals, err := p.storageDealsForPiece(pieceCid != nil, payloadCid, piece)
	if err != nil {
		return retrievalmarket.Ask{}, xerrors.Errorf("failed to fetch deals for payload: %w", err)
	}

	input := retrievalmarket.PricingInput{
		// piece from which the payload will be retrieved
		PieceCID: piece.PieceCID,

		PayloadCID: payloadCid,
		Unsealed:   isUnsealed,
		Client:     client,
	}

	return p.GetDynamicAsk(ctx, input, storageDeals)
}

func (p *AskHandler) storageDealsForPiece(clientSpecificPiece bool, payloadCID cid.Cid, pieceInfo piecestore.PieceInfo) ([]abi.DealID, error) {
	var storageDeals []abi.DealID
	var err error
	if clientSpecificPiece {
		//  If the user wants to retrieve the payload from a specific piece,
		//  we only need to inspect storage deals made for that piece to quote a price.
		for _, d := range pieceInfo.Deals {
			storageDeals = append(storageDeals, d.DealID)
		}
	} else {
		// If the user does NOT want to retrieve from a specific piece, we'll have to inspect all storage deals
		// made for that piece to quote a price.
		storageDeals, err = p.getAllDealsContainingPayload(payloadCID)
		if err != nil {
			return nil, xerrors.Errorf("failed to fetch deals for payload: %w", err)
		}
	}

	if len(storageDeals) == 0 {
		return nil, xerrors.New("no storage deals found")
	}

	return storageDeals, nil
}

func (p *AskHandler) getAllDealsContainingPayload(payloadCID cid.Cid) ([]abi.DealID, error) {
	cidInfo, err := p.pieceStore.GetCIDInfo(payloadCID)
	if err != nil {
		return nil, xerrors.Errorf("get cid info: %w", err)
	}
	var dealsIds []abi.DealID
	var lastErr error

	for _, pieceBlockLocation := range cidInfo.PieceBlockLocations {

		pieceInfo, err := p.pieceStore.GetPieceInfo(pieceBlockLocation.PieceCID)
		if err != nil {
			lastErr = err
			continue
		}
		for _, d := range pieceInfo.Deals {
			dealsIds = append(dealsIds, d.DealID)
		}
	}

	if lastErr == nil && len(dealsIds) == 0 {
		return nil, xerrors.New("no deals found")
	}

	if lastErr != nil && len(dealsIds) == 0 {
		return nil, xerrors.Errorf("failed to fetch deals containing payload %s: %w", payloadCID, lastErr)
	}

	return dealsIds, nil
}
