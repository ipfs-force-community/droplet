package badger

import (
	"github.com/filecoin-project/venus-market/models/itf"
)

type BadgerRepo struct {
	fundRepo         itf.FundRepo
	minerDealRepo    itf.MinerDealRepo
	channelInfoRepo  itf.PaychChannelInfoRepo
	msgInfoRepo      itf.PaychMsgInfoRepo
	storageAskRepo   itf.IStorageAskRepo
	retrievalAskRepo itf.IRetrievalAskRepo
}

func NewBadgerRepo(fundDS itf.FundMgrDS, dealDS itf.ProviderDealDS, paychDS itf.PayChanDS, askDS itf.StorageAskDS, retrAskDs itf.RetrievalAskDS) itf.Repo {
	pst := NewPaychStore(paychDS)

	return &BadgerRepo{fundRepo: NewFundStore(fundDS),
		minerDealRepo:    NewMinerDealStore(dealDS),
		msgInfoRepo:      pst,
		channelInfoRepo:  pst,
		storageAskRepo:   NewAskStore(askDS),
		retrievalAskRepo: NewRetrievalAskRepo(retrAskDs),
	}
}

func (b *BadgerRepo) FundRepo() itf.FundRepo {
	return b.fundRepo
}

func (b *BadgerRepo) MinerDealRepo() itf.MinerDealRepo {
	return b.minerDealRepo
}

func (b *BadgerRepo) PaychMsgInfoRepo() itf.PaychMsgInfoRepo {
	return b.msgInfoRepo
}

func (b *BadgerRepo) PaychChannelInfoRepo() itf.PaychChannelInfoRepo {
	return b.channelInfoRepo
}

func (b *BadgerRepo) StorageAskRepo() itf.IStorageAskRepo {
	return b.storageAskRepo
}

func (r *BadgerRepo) RetrievalAskRepo() itf.IRetrievalAskRepo {
	return r.retrievalAskRepo
}
