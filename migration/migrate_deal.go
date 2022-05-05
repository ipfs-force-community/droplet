package migration

import (
	"github.com/filecoin-project/go-ds-versioning/pkg/versioned"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	logging "github.com/ipfs/go-log/v2"
)

var migrateDealLog = logging.Logger("deal_migrate")

func MigrateMinerDealV0ToV1(oldDeal *types.MinerDealV0) (*types.MinerDeal, error) {
	migrateDealLog.Infof("proposal cid %v", oldDeal.ProposalCid)
	deal := &types.MinerDeal{
		ClientDealProposal:    oldDeal.ClientDealProposal,
		ProposalCid:           oldDeal.ProposalCid,
		AddFundsCid:           oldDeal.AddFundsCid,
		PublishCid:            oldDeal.PublishCid,
		Miner:                 oldDeal.Miner,
		Client:                oldDeal.Client,
		State:                 oldDeal.State,
		PiecePath:             oldDeal.PiecePath,
		PayloadSize:           oldDeal.PayloadSize,
		MetadataPath:          oldDeal.MetadataPath,
		SlashEpoch:            oldDeal.SlashEpoch,
		FastRetrieval:         oldDeal.FastRetrieval,
		Message:               oldDeal.Message,
		FundsReserved:         oldDeal.FundsReserved,
		Ref:                   types.FillDataRef(oldDeal.Ref),
		AvailableForRetrieval: oldDeal.AvailableForRetrieval,
		DealID:                oldDeal.DealID,
		CreationTime:          oldDeal.CreationTime,
		TransferChannelID:     oldDeal.TransferChannelID,
		SectorNumber:          oldDeal.SectorNumber,
		Offset:                oldDeal.Offset,
		PieceStatus:           oldDeal.PieceStatus,
		InboundCAR:            oldDeal.InboundCAR,
	}

	return deal, nil
}

var StorageDealMigrations = versioned.BuilderList{
	versioned.NewVersionedBuilder(MigrateMinerDealV0ToV1, "1"),
}
