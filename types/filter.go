package types

import (
	"context"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
)

type StorageDealFilter func(ctx context.Context, deal storagemarket.MinerDeal) (bool, string, error)
type RetrievalDealFilter func(ctx context.Context, deal retrievalmarket.ProviderDealState) (bool, string, error)
