package idxprov

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/index-provider"
	"github.com/filecoin-project/index-provider/engine"
	"github.com/filecoin-project/index-provider/metadata"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

type IndexProvider struct {
	idxProvider *engine.Engine
	mesh        MeshCreator

	dealStore repo.StorageDealRepo
	dagStore  stores.DAGStoreWrapper
}

var log = logging.Logger("index-provider")

func (ip *IndexProvider) AnnounceIndex(ctx context.Context, deal *types.MinerDeal) (advertCid cid.Cid, err error) {
	mt := metadata.New(&metadata.GraphsyncFilecoinV1{
		PieceCID:      deal.Proposal.PieceCID,
		FastRetrieval: deal.FastRetrieval,
		VerifiedDeal:  deal.Proposal.VerifiedDeal,
	})

	// ensure we have a connection with the full node host so that the index provider gossip sub announcements make their
	// way to the filecoin bootstrapper network
	//if err := ip.Mesh.Connect(ctx); err != nil {
	//	return cid.Undef, fmt.Errorf("cannot publish index record as indexer host failed to connect to the full node: %w", err)
	//}

	return ip.idxProvider.NotifyPut(ctx, deal.ProposalCid.Bytes(), mt)
}

func (ip *IndexProvider) AnnounceLatestAdv(ctx context.Context) (cid.Cid, error) {
	advCid, adv, err := ip.idxProvider.GetLatestAdv(ctx)
	if err != nil {
		return cid.Undef, err
	}

	var publishResultCid cid.Cid
	if publishResultCid, err = ip.idxProvider.Publish(ctx, *adv); err != nil {
		return cid.Undef, err
	}

	log.Infof("manually publish latest advertisement: cid:%s, publish result cid:%s",
		advCid.String(), publishResultCid.String())

	return publishResultCid, nil
}

func (ip *IndexProvider) LatestAdv(ctx context.Context) (cid.Cid, error) {
	cid, _, err := ip.idxProvider.GetLatestAdv(ctx)
	return cid, err
}

func (ip *IndexProvider) start(ctx context.Context) error {
	ip.idxProvider.RegisterMultihashLister(ip.multihashListerCreator)

	if err := ip.idxProvider.Start(ctx); err != nil {
		return fmt.Errorf("start index-provider engine failed:%w", err)
	}

	if _, err := ip.AnnounceLatestAdv(ctx); err != nil {
		return fmt.Errorf("publish latest advertisement failed:%w", err)
	}
	return nil
}

func (ip *IndexProvider) shutdown(ctx context.Context) error {
	return ip.idxProvider.Shutdown()
}

func (ip *IndexProvider) multihashListerCreator(ctx context.Context, contextID []byte) (provider.MultihashIterator, error) {
	proposalCid, err := cid.Cast(contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to cast context ID to a cid")
	}
	deal, err := ip.dealStore.GetDeal(ctx, proposalCid)
	if err != nil {
		return nil, fmt.Errorf("failed getting deal %s: %w", proposalCid, err)
	}
	ii, err := ip.dagStore.GetIterableIndexForPiece(deal.Proposal.PieceCID)
	if err != nil {
		return nil, fmt.Errorf("failed to get iterable index: %w", err)
	}
	mhi, err := provider.CarMultihashIterator(ii)
	if err != nil {
		return nil, fmt.Errorf("failed to get mhiterator: %w", err)
	}
	return mhi, nil
}
