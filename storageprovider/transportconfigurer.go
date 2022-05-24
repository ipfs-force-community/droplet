package storageprovider

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"

	"github.com/filecoin-project/venus-market/v2/models/repo"
)

type providerStoreGetter struct {
	stores *stores.ReadWriteBlockstores
	deals  repo.StorageDealRepo
}

func newProviderStoreGetter(deals repo.StorageDealRepo, stores *stores.ReadWriteBlockstores) *providerStoreGetter {
	return &providerStoreGetter{
		deals:  deals,
		stores: stores,
	}
}

func (psg *providerStoreGetter) Get(proposalCid cid.Cid) (bstore.Blockstore, error) {
	deal, err := psg.deals.GetDeal(context.TODO(), proposalCid)
	if err != nil {
		return nil, fmt.Errorf("failed to get deal state: %w", err)
	}
	return psg.stores.GetOrOpen(proposalCid.String(), deal.InboundCAR, deal.Ref.Root)
}

type providerPushDeals struct {
	deals repo.StorageDealRepo
}

func (ppd *providerPushDeals) Get(proposalCid cid.Cid) (storagemarket.MinerDeal, error) {
	deal, err := ppd.deals.GetDeal(context.TODO(), proposalCid)
	if err != nil {
		return storagemarket.MinerDeal{}, err
	}
	return *deal.FilMarketMinerDeal(), nil
}
