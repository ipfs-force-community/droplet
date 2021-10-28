package badger

import (
	"github.com/filecoin-project/venus-market/models/itf"
)

type BadgerRepo struct {
	fundRepo        itf.FundRepo
	minerDealRepo   itf.MinerDealRepo
	channelInfoRepo itf.PaychChannelInfoRepo
	msgInfoRepo     itf.PaychMsgInfoRepo
	storageAskRepo  itf.IStorageAskRepo
	retrievalRepo   itf.IRetrievalDealRepo
}

func NewBadgerRepo(fundDS itf.FundMgrDS, dealDS itf.ProviderDealDS, retrievalDS itf.RetrievalProviderDS, paychDS itf.PayChanDS, askDS itf.StorageAskDS) itf.Repo {
	pst := NewPaychStore(paychDS)

	return &BadgerRepo{
		fundRepo:        NewFundStore(fundDS),
		minerDealRepo:   NewMinerDealStore(dealDS),
		msgInfoRepo:     pst,
		channelInfoRepo: pst,
		storageAskRepo:  NewAskStore(askDS),
		retrievalRepo:   NewRetrievalDealRepo(retrievalDS),
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

func (b *BadgerRepo) RetrievalDealRepo() itf.IRetrievalDealRepo {
	return b.retrievalRepo
}
