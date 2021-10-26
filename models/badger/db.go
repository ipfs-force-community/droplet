package badger

import (
	"github.com/filecoin-project/venus-market/models/itf"
)

type BadgerRepo struct {
	fundRepo        itf.FundRepo
	minerDealRepo   itf.MinerDealRepo
	channelInfoRepo itf.PaychChannelInfoRepo
	msgInfoRepo     itf.PaychMsgInfoRepo
}

func NewBadgerRepo(ds itf.FundMgrDS) itf.Repo {
	return &BadgerRepo{
		fundRepo: NewFundStore(ds),
	}
}

func (b *BadgerRepo) FundRepo() itf.FundRepo {
	return b.fundRepo
}

func (b *BadgerRepo) MinerParamsRepo() itf.MinerParamsRepo {
	panic("implement me")
}

func (b *BadgerRepo) MinerDealRepo() itf.MinerDealRepo {
	panic("implement me")
}

func (b *BadgerRepo) PaychMsgInfoRepo() itf.PaychMsgInfoRepo {
	panic("implement me")
}
