package retrievaladapter

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
)

type IRetrievalStream interface {
	HandleQueryStream(stream rmnet.RetrievalQueryStream)
}

var _ IRetrievalStream = (*RetrievalStreamHandler)(nil)

type RetrievalStreamHandler struct {
	askHandler         IAskHandler
	retrievalDealStore repo.IRetrievalDealRepo
	storageDealStore   repo.StorageDealRepo
	pieceInfo          *PieceInfo
}

func NewRetrievalStreamHandler(askHandler IAskHandler, retrievalDealStore repo.IRetrievalDealRepo, storageDealStore repo.StorageDealRepo, pieceInfo *PieceInfo) *RetrievalStreamHandler {
	return &RetrievalStreamHandler{askHandler: askHandler, retrievalDealStore: retrievalDealStore, storageDealStore: storageDealStore, pieceInfo: pieceInfo}
}

/*
HandleQueryStream is called by the network implementation whenever a new message is received on the query protocol

A Provider handling a retrieval `Query` does the following:

1. Get the node's chain head in order to get its miner worker address.

2. Look in its piece store to determine if it can serve the given payload CID.

3. Combine these results with its existing parameters for retrieval deals to construct a `retrievalmarket.QueryResponse` struct.

4. Writes this response to the `Query` stream.

The connection is kept open only as long as the query-response exchange.
*/
func (p *RetrievalStreamHandler) HandleQueryStream(stream rmnet.RetrievalQueryStream) {
	ctx, cancel := context.WithTimeout(context.TODO(), queryTimeout)
	defer cancel()

	defer stream.Close()
	query, err := stream.ReadQuery()
	if err != nil {
		return
	}

	sendResp := func(resp retrievalmarket.QueryResponse) {
		if err := stream.WriteQueryResponse(resp); err != nil {
			log.Errorf("Retrieval query: writing query response: %s", err)
		}
	}

	answer := retrievalmarket.QueryResponse{
		Status:          retrievalmarket.QueryResponseUnavailable,
		PieceCIDFound:   retrievalmarket.QueryItemUnavailable,
		MinPricePerByte: big.Zero(),
		UnsealPrice:     big.Zero(),
	}

	// get chain head to query actor states.
	/*tok, _, err := p.node.GetChainHead(ctx)
	if err != nil {
		log.Errorf("Retrieval query: GetChainHead: %s", err)
		return
	}

	// fetch the payment address the client should send the payment to.
	paymentAddress, err := p.node.GetMinerWorkerAddress(ctx, p.minerAddress, tok)
	if err != nil {
		log.Errorf("Retrieval query: Lookup Payment Address: %s", err)
		answer.Status = retrievalmarket.QueryResponseError
		answer.Message = fmt.Sprintf("failed to look up payment address: %s", err)
		sendResp(answer)
		return
	}*/
	//todo payment address
	answer.PaymentAddress = address.Undef

	// fetch the piece from which the payload will be retrieved.
	// if user has specified the Piece in the request, we use that.
	// Otherwise, we prefer a Piece which can retrieved from an unsealed sector.
	pieceCID := cid.Undef
	if query.PieceCID != nil {
		pieceCID = *query.PieceCID
	}
	pieceInfo, isUnsealed, err := p.pieceInfo.GetPieceInfoFromCid(ctx, query.PayloadCID, pieceCID)
	if err != nil {
		log.Errorf("Retrieval query: getPieceInfoFromCid: %s", err)
		if !xerrors.Is(err, retrievalmarket.ErrNotFound) {
			answer.Status = retrievalmarket.QueryResponseError
			answer.Message = fmt.Sprintf("failed to fetch piece to retrieve from: %s", err)
		}

		sendResp(answer)
		return
	}

	answer.Status = retrievalmarket.QueryResponseAvailable
	answer.Size = uint64(pieceInfo.Deals[0].Length.Unpadded()) // TODO: verify on intermediate
	answer.PieceCIDFound = retrievalmarket.QueryItemAvailable

	ask, err := p.askHandler.GetAskForPayload(ctx, query.PayloadCID, query.PieceCID, pieceInfo, isUnsealed, stream.RemotePeer())
	if err != nil {
		log.Errorf("Retrieval query: GetAsk: %s", err)
		answer.Status = retrievalmarket.QueryResponseError
		answer.Message = fmt.Sprintf("failed to price deal: %s", err)
		sendResp(answer)
		return
	}

	answer.MinPricePerByte = ask.PricePerByte
	answer.MaxPaymentInterval = ask.PaymentInterval
	answer.MaxPaymentIntervalIncrease = ask.PaymentIntervalIncrease
	answer.UnsealPrice = ask.UnsealPrice
	sendResp(answer)
}
