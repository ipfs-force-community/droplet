package badger

import (
	"github.com/filecoin-project/venus-market/models/itf"
)

type BadgerRepo struct {
	fundRepo         itf.FundRepo
	storageDealRepo  itf.StorageDealRepo
	channelInfoRepo  itf.PaychChannelInfoRepo
	msgInfoRepo      itf.PaychMsgInfoRepo
	storageAskRepo   itf.IStorageAskRepo
	retrievalAskRepo itf.IRetrievalAskRepo
	piecesRepo       itf.IPieceRepo
	retrievalRepo    itf.IRetrievalDealRepo
}

func NewBadgerRepo(fundDS itf.FundMgrDS, dealDS itf.ProviderDealDS,
	paychDS itf.PayChanDS, askDS itf.StorageAskDS, retrAskDs itf.RetrievalAskDS,
	pieceDs itf.PieceInfoDS, cidInfoDs itf.CIDInfoDS,
	retrievalDs itf.RetrievalProviderDS) (itf.Repo, error) {
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

func (r *BadgerRepo) FundRepo() itf.FundRepo {
	return r.fundRepo
}

func (r *BadgerRepo) StorageDealRepo() itf.StorageDealRepo {
	return r.storageDealRepo
}

func (r *BadgerRepo) PaychMsgInfoRepo() itf.PaychMsgInfoRepo {
	return r.msgInfoRepo
}

func (r *BadgerRepo) PaychChannelInfoRepo() itf.PaychChannelInfoRepo {
	return r.channelInfoRepo
}

func (r *BadgerRepo) StorageAskRepo() itf.IStorageAskRepo {
	return r.storageAskRepo
}

func (b *BadgerRepo) RetrievalAskRepo() itf.IRetrievalAskRepo {
	return b.retrievalAskRepo
}

func (b *BadgerRepo) PieceRepo() itf.IPieceRepo {
	return b.piecesRepo
}

func (r *BadgerRepo) RetrievalDealRepo() itf.IRetrievalDealRepo {
	return r.retrievalRepo
}
