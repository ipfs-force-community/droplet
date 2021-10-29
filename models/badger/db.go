package badger

import (
	"github.com/filecoin-project/venus-market/models/repo"
)

type BadgerRepo struct {
	fundRepo         repo.FundRepo
	storageDealRepo  repo.StorageDealRepo
	channelInfoRepo  repo.PaychChannelInfoRepo
	msgInfoRepo      repo.PaychMsgInfoRepo
	storageAskRepo   repo.IStorageAskRepo
	retrievalAskRepo repo.IRetrievalAskRepo
	piecesRepo       repo.IPieceRepo
	retrievalRepo    repo.IRetrievalDealRepo
}

func NewBadgerRepo(fundDS repo.FundMgrDS, dealDS repo.ProviderDealDS,
	paychDS repo.PayChanDS, askDS repo.StorageAskDS, retrAskDs repo.RetrievalAskDS,
	pieceDs repo.PieceInfoDS, cidInfoDs repo.CIDInfoDS,
	retrievalDs repo.RetrievalProviderDS) (repo.Repo, error) {
	pst := NewPaychRepo(paychDS)

	pieceRepo, err := NewBadgerPieceRepo(pieceDs, cidInfoDs)
	if err != nil {
		return nil, err
	}

	return &BadgerRepo{
		fundRepo:         NewFundRepo(fundDS),
		storageDealRepo:  NewStorageDealRepo(dealDS),
		msgInfoRepo:      pst,
		channelInfoRepo:  pst,
		storageAskRepo:   NewStorageAskRepo(askDS),
		retrievalAskRepo: NewRetrievalAskRepo(retrAskDs),
		piecesRepo:       pieceRepo,
		retrievalRepo:    NewRetrievalDealRepo(retrievalDs),
	}, nil
}

func (r *BadgerRepo) FundRepo() repo.FundRepo {
	return r.fundRepo
}

func (r *BadgerRepo) StorageDealRepo() repo.StorageDealRepo {
	return r.storageDealRepo
}

func (r *BadgerRepo) PaychMsgInfoRepo() repo.PaychMsgInfoRepo {
	return r.msgInfoRepo
}

func (r *BadgerRepo) PaychChannelInfoRepo() repo.PaychChannelInfoRepo {
	return r.channelInfoRepo
}

func (r *BadgerRepo) StorageAskRepo() repo.IStorageAskRepo {
	return r.storageAskRepo
}

func (b *BadgerRepo) RetrievalAskRepo() repo.IRetrievalAskRepo {
	return b.retrievalAskRepo
}

func (b *BadgerRepo) PieceRepo() repo.IPieceRepo {
	return b.piecesRepo
}

func (r *BadgerRepo) RetrievalDealRepo() repo.IRetrievalDealRepo {
	return r.retrievalRepo
}
