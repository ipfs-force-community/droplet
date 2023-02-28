package v0api

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"

	v1API "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/types/market"
)

type WrapperV1IMarket struct {
	v1API.IMarket
}

func (w WrapperV1IMarket) UpdateDealStatus(ctx context.Context, miner address.Address, dealID abi.DealID, pieceStatus market.PieceStatus) error {
	return w.IMarket.UpdateDealStatus(ctx, miner, dealID, pieceStatus, storagemarket.StorageDealAwaitingPreCommit)
}
