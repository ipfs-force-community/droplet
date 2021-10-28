package storageadapter

import (
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"
)

type providerStoreGetter struct {
	stores *stores.ReadWriteBlockstores
	deals  MinerDealStore
}

func newProviderStoreGetter(deals MinerDealStore) *providerStoreGetter {
	return &providerStoreGetter{
		deals:  deals,
		stores: stores.NewReadWriteBlockstores(),
	}
}

func (psg *providerStoreGetter) Get(proposalCid cid.Cid) (bstore.Blockstore, error) {
	deal, err := psg.deals.Get(proposalCid)
	if err != nil {
		return nil, xerrors.Errorf("failed to get deal state: %w", err)
	}
	return psg.stores.GetOrOpen(proposalCid.String(), deal.InboundCAR, deal.Ref.Root)
}

type providerPushDeals struct {
	deals MinerDealStore
}

func (ppd *providerPushDeals) Get(proposalCid cid.Cid) (storagemarket.MinerDeal, error) {
	deal, err := ppd.deals.Get(proposalCid)
	if err != nil {
		return storagemarket.MinerDeal{}, err
	}
	return *deal, nil
}
