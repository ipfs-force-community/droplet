package migrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	versioning "github.com/filecoin-project/go-ds-versioning/pkg"
	"github.com/filecoin-project/go-ds-versioning/pkg/statestore"
	"github.com/filecoin-project/go-ds-versioning/pkg/versioned"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	v220 "github.com/filecoin-project/venus-market/v2/models/badger/migrate/v2.2.0"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
)

const (
	DsNameFundedAddrState  = "FundedAddrStateDs"
	DsNameStorageDeal      = "StorageDealDs"
	DsNamePaychInfoDs      = "PayChanInfoDs"
	DsNamePaychMsgDs       = "PayChanMsgDs"
	DsNameStorageAskDs     = "StorageAskDs"
	DsNameRetrievalAskDs   = "RetrievalAskDs"
	DsNameCidInfoDs        = "CidInfoDs"
	DsNameRetrievalDealsDs = "RetrievalDealsDS"
)

var versioningKey = datastore.NewKey("/versions/current")

var log = logging.Logger("badger-migration")

type migrateFunc struct {
	version versioning.VersionKey
	mf      versioning.MigrationFunc
}

type migrateFuncSchedule []migrateFunc

func (mfs migrateFuncSchedule) targetVersion() versioning.VersionKey {
	return mfs[len(mfs)-1].version
}

func (mfs migrateFuncSchedule) subScheduleFrom(fromVersion string) migrateFuncSchedule {
	if len(fromVersion) == 0 {
		return mfs
	}
	var startPos int
	for idx, mf := range mfs {
		if string(mf.version) == fromVersion {
			startPos = idx
			break
		}
	}
	if startPos == 0 {
		return nil
	}
	return mfs[startPos:]
}

func timeStampNow() market.TimeStamp {
	ts := time.Now().Unix()
	return market.TimeStamp{CreatedAt: uint64(ts), UpdatedAt: uint64(ts)}
}

var migrateDs = map[string][]migrateFunc{
	DsNameFundedAddrState: {
		{version: "1", mf: func(old *v220.FundedAddressState) (*market.FundedAddressState, error) {
			return &market.FundedAddressState{
				Addr:        old.Addr,
				AmtReserved: old.AmtReserved,
				MsgCid:      old.MsgCid,
			}, nil
		}},
	},
	DsNameStorageDeal: {
		{version: "1", mf: func(old *v220.MinerDeal) (*market.MinerDeal, error) {
			return &market.MinerDeal{
				ClientDealProposal:    old.ClientDealProposal,
				ProposalCid:           old.ProposalCid,
				AddFundsCid:           old.AddFundsCid,
				PublishCid:            old.PublishCid,
				Miner:                 old.Miner,
				Client:                old.Client,
				State:                 old.State,
				PiecePath:             old.PiecePath,
				PayloadSize:           old.PayloadSize,
				MetadataPath:          old.MetadataPath,
				SlashEpoch:            old.SlashEpoch,
				FastRetrieval:         old.FastRetrieval,
				Message:               old.Message,
				FundsReserved:         old.FundsReserved,
				Ref:                   old.Ref,
				AvailableForRetrieval: old.AvailableForRetrieval,
				DealID:                old.DealID,
				CreationTime:          old.CreationTime,
				TransferChannelID:     old.TransferChannelID,
				SectorNumber:          old.SectorNumber,
				Offset:                old.Offset,
				PieceStatus:           market.PieceStatus(old.PieceStatus),
				InboundCAR:            old.InboundCAR,
				TimeStamp:             timeStampNow(),
			}, nil
		},
		},
	},
	DsNamePaychInfoDs: {
		{version: "1", mf: func(old *v220.ChannelInfo) (*market.ChannelInfo, error) {
			info := &market.ChannelInfo{
				ChannelID: old.ChannelID,
				Channel:   old.Channel,
				Control:   old.Control,
				Target:    old.Target,
				Direction: old.Direction,
				//Vouchers:      old.Vouchers,
				NextLane:      old.NextLane,
				Amount:        old.Amount,
				PendingAmount: old.PendingAmount,
				CreateMsg:     old.CreateMsg,
				AddFundsMsg:   old.AddFundsMsg,
				Settling:      old.Settling,
				TimeStamp:     timeStampNow(),
			}
			if len(old.Vouchers) == 0 {
				return info, nil
			}

			info.Vouchers = make([]*market.VoucherInfo, len(old.Vouchers))
			for idx, vch := range old.Vouchers {
				info.Vouchers[idx] = &market.VoucherInfo{
					Voucher:   vch.Voucher,
					Proof:     vch.Proof,
					Submitted: vch.Submitted,
				}
			}
			return info, nil
		},
		},
	},
	DsNamePaychMsgDs: {
		{version: "1", mf: func(old *v220.MsgInfo) (*market.MsgInfo, error) {
			return &market.MsgInfo{
				ChannelID: old.ChannelID,
				MsgCid:    old.MsgCid,
				Received:  old.Received,
				Err:       old.Err,
				TimeStamp: timeStampNow(),
			}, nil
		}},
	},
	DsNameStorageAskDs: {
		{version: "1", mf: func(old *v220.SignedStorageAsk) (*market.SignedStorageAsk, error) {
			return &market.SignedStorageAsk{
				Ask:       old.Ask,
				Signature: old.Signature,
				TimeStamp: timeStampNow(),
			}, nil
		}},
	},
	DsNameRetrievalAskDs: {
		{version: "1", mf: func(old *v220.RetrievalAsk) (*market.RetrievalAsk, error) {
			return &market.RetrievalAsk{
				Miner:                   old.Miner,
				PricePerByte:            old.PricePerByte,
				UnsealPrice:             old.UnsealPrice,
				PaymentInterval:         old.PaymentInterval,
				PaymentIntervalIncrease: old.PaymentIntervalIncrease,
				TimeStamp:               timeStampNow()}, nil
		}},
	},
	DsNameCidInfoDs: {
		{version: "1", mf: func(old *v220.CIDInfo) (*piecestore.CIDInfo, error) {
			return &piecestore.CIDInfo{
				CID:                 old.CID,
				PieceBlockLocations: old.PieceBlockLocations,
			}, nil
		}},
	},
	DsNameRetrievalDealsDs: {
		{version: "1", mf: func(old *v220.ProviderDealState) (*market.ProviderDealState, error) {
			return &market.ProviderDealState{
				DealProposal:          old.DealProposal,
				StoreID:               old.StoreID,
				SelStorageProposalCid: old.SelStorageProposalCid,
				ChannelID:             old.ChannelID,
				Status:                old.Status,
				Receiver:              old.Receiver,
				TotalSent:             old.TotalSent,
				FundsReceived:         old.FundsReceived,
				Message:               old.Message,
				CurrentInterval:       old.CurrentInterval,
				LegacyProtocol:        old.LegacyProtocol,
				TimeStamp:             timeStampNow()}, nil
		}},
	},
}

func migrateOne(ctx context.Context, name string, mfs migrateFuncSchedule, ds datastore.Batching) (datastore.Batching, error) {
	var oldVersion string
	if v, err := ds.Get(ctx, versioningKey); err != nil {
		if !errors.Is(err, datastore.ErrNotFound) {
			return nil, err
		}
	} else {
		oldVersion = string(v)
	}

	var targetVersion = mfs.targetVersion()

	var dsWithOldVersion datastore.Batching

	if len(oldVersion) == 0 {
		dsWithOldVersion = ds
	} else {
		dsWithOldVersion = namespace.Wrap(ds, datastore.NewKey(oldVersion))

		if oldVersion == string(targetVersion) {
			log.Infof("doesn't need migration for %s, current version is:%s", name, oldVersion)
			return dsWithOldVersion, nil
		}
	}

	log.Infof("migrate: %s from %s to %s", name, oldVersion, string(targetVersion))

	mfs = mfs.subScheduleFrom(oldVersion)

	if len(mfs) == 0 {
		return nil, fmt.Errorf("migrate:%s failed, can't find schedule from:%s", name, oldVersion)
	}

	var migrationBuilders versioned.BuilderList = make([]versioned.Builder, len(mfs))

	for idx, mf := range mfs {
		migrationBuilders[idx] = versioned.NewVersionedBuilder(mf.mf, mf.version)
	}

	migrations, err := migrationBuilders.Build()
	if err != nil {
		return nil, err
	}

	_, doMigrate := statestore.NewVersionedStateStore(dsWithOldVersion, migrations, targetVersion)
	if err := doMigrate(ctx); err != nil {
		var rollbackErr error

		// if error happens, just rollback the version number
		if len(oldVersion) == 0 {
			rollbackErr = ds.Delete(ctx, versioningKey)
		} else {
			rollbackErr = ds.Put(ctx, versioningKey, []byte(oldVersion))
		}

		// there are nothing we can do to get back the data.
		if rollbackErr != nil {
			log.Errorf("migrate: %s failed, rollback version failed:%v\n", name, rollbackErr)
		}
		return nil, err
	}

	return namespace.Wrap(ds, datastore.NewKey(string(targetVersion))), nil
}

func Migrate(ctx context.Context, dss map[string]datastore.Batching) (map[string]datastore.Batching, error) {
	var err error
	for name, ds := range dss {
		mfs, exist := migrateDs[name]

		if !exist {
			dss[name] = ds
			log.Warnf("no migration sechedules for : %s", name)
			continue
		}

		var versionedDs datastore.Batching

		versionedDs, err = migrateOne(ctx, name, mfs, ds)

		// todo: version为空同时, 有同时存在两个版本的类型的可能性, 为了兼容, 这里暂时不返回错误.
		//  后续的版本升级中如果出错, 应该直接返回.
		if err != nil {
			dss[name] = ds
			log.Warnf("migrate:%s failed:%s", name, err.Error())
			continue
		}
		dss[name] = versionedDs
	}
	return dss, nil
}
