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
	retrievalRepo    itf.IRetrievalDealRepo
}

func NewBadgerRepo(fundDS itf.FundMgrDS, dealDS itf.ProviderDealDS, retrievalDS itf.RetrievalProviderDS, paychDS itf.PayChanDS, askDS itf.StorageAskDS, retrAskDs itf.RetrievalAskDS) itf.Repo {
	pst := NewPaychRepo(paychDS)

	return &BadgerRepo{
		fundRepo:         NewFundRepo(fundDS),
		storageDealRepo:  NewStorageDealRepo(dealDS),
		msgInfoRepo:      pst,
		channelInfoRepo:  pst,
		storageAskRepo:   NewStorageAskRepo(askDS),
		retrievalAskRepo: NewRetrievalAskRepo(retrAskDs),
		retrievalRepo:    NewRetrievalDealRepo(retrievalDS),
	}
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

func (r *BadgerRepo) RetrievalDealRepo() itf.IRetrievalDealRepo {
	return r.retrievalRepo
}

func (r *BadgerRepo) RetrievalAskRepo() itf.IRetrievalAskRepo {
	return r.retrievalAskRepo
}
