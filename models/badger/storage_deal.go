package badger

import (
	"bytes"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/types"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

type storageDealRepo struct {
	ds datastore.Batching
}

var _ (repo.StorageDealRepo) = (*storageDealRepo)(nil)

func NewStorageDealRepo(ds repo.ProviderDealDS) *storageDealRepo {
	return &storageDealRepo{ds}
}

func (sdr *storageDealRepo) SaveStorageDeal(storageDeal *types.MinerDeal) error {
	b, err := cborrpc.Dump(storageDeal)
	if err != nil {
		return err
	}
	return sdr.ds.Put(statestore.ToKey(storageDeal.ProposalCid), b)
}

func (sdr *storageDealRepo) GetStorageDeal(proposalCid cid.Cid) (*types.MinerDeal, error) {
	value, err := sdr.ds.Get(statestore.ToKey(proposalCid))
	if err != nil {
		return nil, err
	}
	var deal types.MinerDeal
	if err = cborrpc.ReadCborRPC(bytes.NewReader(value), &deal); err != nil {
		return nil, err
	}

	return &deal, nil
}

func (sdr *storageDealRepo) ListStorageDeal() ([]*types.MinerDeal, error) {
	storageDeals := make([]*types.MinerDeal, 0)
	if err := sdr.travelDeals(func(deal *types.MinerDeal) error {
		storageDeals = append(storageDeals, deal)
		return nil
	}); err != nil {
		return nil, err
	}
	return storageDeals, nil
}

func (m *storageDealRepo) GetPieceInfo(pieceCID cid.Cid) (*piecestore.PieceInfo, error) {
	var pieceInfo = piecestore.PieceInfo{
		PieceCID: pieceCID,
		Deals:    nil,
	}

	if err := m.travelDeals(func(deal *types.MinerDeal) error {
		if deal.ClientDealProposal.Proposal.PieceCID.Equals(pieceCID) {
			pieceInfo.Deals = append(pieceInfo.Deals, piecestore.DealInfo{
				DealID:   deal.DealID,
				SectorID: deal.SectorNumber,
				Offset:   deal.Offset,
				Length:   deal.Length,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &pieceInfo, nil
}

func (dsr *storageDealRepo) travelDeals(travelFn func(deal *types.MinerDeal) error) error {
	result, err := dsr.ds.Query(query.Query{})
	if err != nil {
		return err
	}
	defer result.Close() //nolint:errcheck

	for res := range result.Next() {
		if res.Error != nil {
			return err
		}
		var deal types.MinerDeal
		if err = cborrpc.ReadCborRPC(bytes.NewReader(res.Value), &deal); err != nil {
			return err
		}
		if err = travelFn(&deal); err != nil {
			return err
		}
	}
	return nil
}
