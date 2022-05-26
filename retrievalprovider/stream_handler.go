package retrievalprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/v2/models/repo"
)

type IRetrievalStream interface {
	HandleQueryStream(stream rmnet.RetrievalQueryStream)
}

var _ IRetrievalStream = (*RetrievalStreamHandler)(nil)

type RetrievalStreamHandler struct {
	askRepo            repo.IRetrievalAskRepo
	retrievalDealStore repo.IRetrievalDealRepo
	storageDealStore   repo.StorageDealRepo
	pieceInfo          *PieceInfo
	paymentAddr        address.Address
}

func NewRetrievalStreamHandler(askRepo repo.IRetrievalAskRepo, retrievalDealStore repo.IRetrievalDealRepo, storageDealStore repo.StorageDealRepo, pieceInfo *PieceInfo, paymentAddr address.Address) *RetrievalStreamHandler {
	return &RetrievalStreamHandler{askRepo: askRepo, retrievalDealStore: retrievalDealStore, storageDealStore: storageDealStore, pieceInfo: pieceInfo, paymentAddr: paymentAddr}
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

	defer func() {
		if err := stream.Close(); err != nil {
			log.Errorf("unable to close stream %v", err)
		}
	}()
	query, err := stream.ReadQuery()
	if err != nil {
		return
	}

	// since 'paymentAddr' is empty, to `cborMarshal` a struct with empty address would get an error,
	// it is impossible to `WriteQueryResponse` success.
	// just return and output a log.
	if p.paymentAddr == address.Undef {
		log.Errorf("'RetrievalPaymentAddress' configuration is not set")
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
		PaymentAddress:  p.paymentAddr,
	}

	minerDeals, err := p.pieceInfo.GetPieceInfoFromCid(ctx, query.PayloadCID, query.PieceCID)
	if err != nil {
		log.Errorf("Retrieval query: query ready data: %s", err)
		answer.Status = retrievalmarket.QueryResponseError
		if errors.Is(err, repo.ErrNotFound) {
			answer.Message = fmt.Sprintf("retrieve piece(%s) or payload(%s) failed, not found",
				query.PieceCID, query.PayloadCID)
		} else {
			answer.Message = fmt.Sprintf("failed to fetch piece to retrieve from: %s", err)
		}
		sendResp(answer)
		return
	}

	answer.Status = retrievalmarket.QueryResponseAvailable
	//todo payload size maybe different with real piece size.
	answer.Size = uint64(minerDeals[0].Proposal.PieceSize.Unpadded()) // TODO: verify on intermediate
	answer.PieceCIDFound = retrievalmarket.QueryItemAvailable
	answer.PaymentAddress = p.paymentAddr

	//todo use market ask maybe need miner ask list for future
	ask, err := p.askRepo.GetAsk(ctx, p.paymentAddr)
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
