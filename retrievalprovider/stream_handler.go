package retrievalprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-market/v2/config"

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
	cfg                *config.MarketConfig
	askRepo            repo.IRetrievalAskRepo
	retrievalDealStore repo.IRetrievalDealRepo
	storageDealStore   repo.StorageDealRepo
	pieceInfo          *PieceInfo
}

func NewRetrievalStreamHandler(cfg *config.MarketConfig, askRepo repo.IRetrievalAskRepo, retrievalDealStore repo.IRetrievalDealRepo, storageDealStore repo.StorageDealRepo, pieceInfo *PieceInfo) *RetrievalStreamHandler {
	return &RetrievalStreamHandler{cfg: cfg, askRepo: askRepo, retrievalDealStore: retrievalDealStore, storageDealStore: storageDealStore, pieceInfo: pieceInfo}
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

	sendResp := func(resp retrievalmarket.QueryResponse) {
		if resp.Status == retrievalmarket.QueryResponseError {
			log.Errorf(resp.Message)
		}
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

	minerDeals, err := p.pieceInfo.GetPieceInfoFromCid(ctx, query.PayloadCID, query.PieceCID)
	if err != nil {
		answer.Status = retrievalmarket.QueryResponseError
		if errors.Is(err, repo.ErrNotFound) {
			answer.Message = fmt.Sprintf("retrieve piece(%v) or payload(%s) failed, not found",
				query.PieceCID, query.PayloadCID)
		} else {
			answer.Message = fmt.Sprintf("failed to fetch piece to retrieve from: %s", err)
		}
		sendResp(answer)
		return
	}

	log := log.With("payload cid", query.PayloadCID)
	for _, deal := range minerDeals {
		answer.Status = retrievalmarket.QueryResponseAvailable
		// todo payload size maybe different with real piece size.
		answer.Size = uint64(deal.Proposal.PieceSize.Unpadded()) // TODO: verify on intermediate
		answer.PieceCIDFound = retrievalmarket.QueryItemAvailable

		minerCfg, err := p.cfg.MinerProviderConfig(deal.Proposal.Provider, true)
		if err != nil {
			log.Warn(err)
			continue
		}
		paymentAddr := minerCfg.RetrievalPaymentAddress.Unwrap()
		if paymentAddr == address.Undef {
			log.Warnf("must specify payment address")
			continue
		}
		answer.PaymentAddress = paymentAddr

		ask, err := p.askRepo.GetAsk(ctx, deal.Proposal.Provider)
		if err != nil {
			log.Warnf("got %s ask failed: %v", deal.Proposal.Provider, err)
			continue
		}
		answer.MinPricePerByte = ask.PricePerByte
		answer.MaxPaymentInterval = ask.PaymentInterval
		answer.MaxPaymentIntervalIncrease = ask.PaymentIntervalIncrease
		answer.UnsealPrice = ask.UnsealPrice

		sendResp(answer)
		return
	}

	sendResp(retrievalmarket.QueryResponse{
		Status:  retrievalmarket.QueryResponseError,
		Message: "failed to query deal",
	})
}
