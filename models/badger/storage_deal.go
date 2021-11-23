package badger

import (
	"bytes"
	"errors"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/types"
	"golang.org/x/xerrors"

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

func NewStorageDealRepo(ds ProviderDealDS) *storageDealRepo {
	return &storageDealRepo{ds}
}

func (sdr *storageDealRepo) SaveDeal(storageDeal *types.MinerDeal) error {
	b, err := cborrpc.Dump(storageDeal)
	if err != nil {
		return err
	}
	return sdr.ds.Put(statestore.ToKey(storageDeal.ProposalCid), b)
}

func (sdr *storageDealRepo) GetDeal(proposalCid cid.Cid) (*types.MinerDeal, error) {
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

func (sdr *storageDealRepo) GetDeals(miner address.Address, pageIndex, pageSize int) ([]*types.MinerDeal, error) {
	var startIdx, idx = pageIndex * pageSize, 0
	var storageDeals []*types.MinerDeal
	var err error
	if err = sdr.travelDeals(func(deal *types.MinerDeal) (err error) {
		if deal.ClientDealProposal.Proposal.Provider != miner {
			return
		}
		idx++
		if idx-1 < startIdx {
			return
		}
		storageDeals = append(storageDeals, deal)
		if len(storageDeals) >= pageSize {
			return justWantStopTravelErr
		}
		return
	}); err != nil {
		if xerrors.Is(err, justWantStopTravelErr) {
			return storageDeals, nil
		}
		return nil, err
	}

	if len(storageDeals) == 0 {
		err = repo.ErrNotFound
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) GetDealsByPieceCidAndStatus(piececid cid.Cid, statues []storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	filter := map[storagemarket.StorageDealStatus]struct{}{}
	for _, status := range statues {
		filter[status] = struct{}{}
	}

	var storageDeals []*types.MinerDeal
	var err error
	if err = sdr.travelDeals(func(deal *types.MinerDeal) (err error) {
		if deal.ClientDealProposal.Proposal.PieceCID != piececid {
			return
		}

		if _, ok := filter[deal.State]; !ok {
			return
		}

		storageDeals = append(storageDeals, deal)
		return
	}); err != nil {
		if xerrors.Is(err, justWantStopTravelErr) {
			return storageDeals, nil
		}
		return nil, err
	}

	if len(storageDeals) == 0 {
		err = repo.ErrNotFound
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) GetDealByAddrAndStatus(addr address.Address, status storagemarket.StorageDealStatus) ([]*types.MinerDeal, error) {
	var storageDeals []*types.MinerDeal
	var err error
	if err = sdr.travelDeals(
		func(deal *types.MinerDeal) (err error) {
		if deal.ClientDealProposal.Proposal.Provider == addr && deal.State == status {
			storageDeals = append(storageDeals, deal)
		}
		return nil
	}); err != nil {
		if xerrors.Is(err, justWantStopTravelErr) {
			return storageDeals, nil
		}
		return nil, err
	}

	return storageDeals, err
}

func (sdr *storageDealRepo) UpdateDealStatus(proposalCid cid.Cid, status storagemarket.StorageDealStatus) error {
	deal, err := sdr.GetDeal(proposalCid)
	if err != nil {
		return err
	}
	deal.State = status
	return sdr.SaveDeal(deal)
}

func (sdr *storageDealRepo) ListDeal(miner address.Address) ([]*types.MinerDeal, error) {
	storageDeals := make([]*types.MinerDeal, 0)
	if err := sdr.travelDeals(func(deal *types.MinerDeal) (err error) {
		if deal.ClientDealProposal.Proposal.Provider == miner {
			storageDeals = append(storageDeals, deal)
		}
		return
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
				Length:   deal.Proposal.PieceSize,
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

func (dsr *storageDealRepo) ListPieceInfoKeys() ([]cid.Cid, error) {
	var cidsMap = make(map[cid.Cid]interface{})
	err := dsr.travelDeals(
		func(deal *types.MinerDeal) error {
			cidsMap[deal.ClientDealProposal.Proposal.PieceCID] = nil
			return nil
		})
	if err != nil {
		return nil, err
	}

	cids := make([]cid.Cid, len(cidsMap))
	idx := 0
	for cid := range cidsMap {
		cids[idx] = cid
		idx++
	}
	return cids, nil
}

var justWantStopTravelErr = errors.New("stop travel")

func (dsr *storageDealRepo) GetDealByDealID(mAddr address.Address, dealID abi.DealID) (*types.MinerDeal, error) {
	var deal *types.MinerDeal
	var err error
	if err = dsr.travelDeals(
		func(inDeal *types.MinerDeal) error {
			if inDeal.ClientDealProposal.Proposal.Provider == mAddr && inDeal.DealID == dealID {
				deal = inDeal
				return xerrors.Errorf("find the deal, so stop:%w", justWantStopTravelErr)
			}
			return nil
		}); err != nil {
		if xerrors.Is(err, justWantStopTravelErr) {
			return deal, nil
		}
		return nil, err
	}
	return nil, repo.ErrNotFound
}

func (dsr *storageDealRepo) GetDealsByPieceStatus(mAddr address.Address, pieceStatus string) ([]*types.MinerDeal, error) {
	var deals []*types.MinerDeal
	var err error
	if err = dsr.travelDeals(
		func(inDeal *types.MinerDeal) error {
			if inDeal.ClientDealProposal.Proposal.Provider == mAddr && inDeal.PieceStatus == pieceStatus {
				deals = append(deals, inDeal)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	return deals, nil
}
