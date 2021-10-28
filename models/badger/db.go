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
	piecesRepo       itf.IPieceRepo
}

func NewBadgerRepo(fundDS itf.FundMgrDS, dealDS itf.ProviderDealDS, paychDS itf.PayChanDS, askDS itf.StorageAskDS, retrAskDs itf.RetrievalAskDS, pieceDs itf.PieceInfoDS, cidInfoDs itf.CIDInfoDS) (itf.Repo, error) {
	pst := NewPaychStore(paychDS)

	pieceRepo, err := NewBadgerPieceRepo(pieceDs, cidInfoDs)
	if err != nil {
		return nil, err
	}

	return &BadgerRepo{fundRepo: NewFundStore(fundDS),
		minerDealRepo:    NewMinerDealStore(dealDS),
		msgInfoRepo:      pst,
		channelInfoRepo:  pst,
		storageAskRepo:   NewAskStore(askDS),
		retrievalAskRepo: NewRetrievalAskRepo(retrAskDs),
		piecesRepo:       pieceRepo}, nil
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

func (b *BadgerRepo) RetrievalAskRepo() itf.IRetrievalAskRepo {
	return b.retrievalAskRepo
}

func (b *BadgerRepo) PieceRepo() itf.IPieceRepo {
	return b.piecesRepo
}
