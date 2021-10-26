package badger

import (
	"github.com/filecoin-project/venus-market/models"
)

type BadgerRepo struct {
	fundRepo        models.FundRepo
	minerDealRepo   models.MinerDealRepo
	channelInfoRepo models.PaychChannelInfoRepo
	msgInfoRepo     models.PaychMsgInfoRepo
}

func NewBadgerRepo(ds models.FundMgrDS) models.Repo {
	return &BadgerRepo{
		fundRepo: NewFundStore(ds),
	}
}

func (b *BadgerRepo) FundRepo() models.FundRepo {
	return b.fundRepo
}

func (b *BadgerRepo) MinerParamsRepo() models.MinerParamsRepo {
	panic("implement me")
}

func (b *BadgerRepo) MinerDealRepo() models.MinerDealRepo {
	panic("implement me")
}

func (b *BadgerRepo) PaychMsgInfoRepo() models.PaychMsgInfoRepo {
	panic("implement me")
}
