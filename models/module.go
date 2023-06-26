package models

import (
	badger2 "github.com/ipfs-force-community/droplet/v2/models/badger"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	"github.com/ipfs-force-community/droplet/v2/models/repo"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/venus-common-utils/builder"
)

// TODO: 这里没有考虑client和server的数据表是不一样的
var DBOptions = func(server bool, mysqlCfg *config.Mysql) builder.Option {
	return builder.Options(
		builder.Override(new(badger2.MetadataDS), badger2.NewMetadataDS),
		builder.ApplyIfElse(func(s *builder.Settings) bool {
			return server
		}, builder.Options(
			// if server, use mysql
			builder.Override(new(badger2.StagingDS), badger2.NewStagingDS),
			builder.Override(new(badger2.StagingBlockstore), badger2.NewStagingBlockStore),
			builder.Override(new(badger2.DagTransferDS), badger2.NewDagTransferDS),
			builder.ApplyIfElse(func(s *builder.Settings) bool {
				return mysqlCfg != nil && len(mysqlCfg.ConnectionString) > 0
			}, builder.Options(
				// if mysql is configured, use mysql
				builder.Override(new(repo.Repo), func() (repo.Repo, error) {
					return mysql.NewMysqlRepo(mysqlCfg)
				}),
			),
				builder.Options(
					// if mysql is not configured, use badger
					builder.Override(new(badger2.PieceMetaDs), badger2.NewPieceMetaDs),
					builder.Override(new(badger2.CIDInfoDS), badger2.NewCidInfoDs),
					builder.Override(new(badger2.RetrievalProviderDS), badger2.NewRetrievalProviderDS),
					builder.Override(new(badger2.RetrievalAskDS), badger2.NewRetrievalAskDS),
					builder.Override(new(badger2.StorageProviderDS), badger2.NewStorageProviderDS),
					builder.Override(new(badger2.StorageDealsDS), badger2.NewStorageDealsDS),
					builder.Override(new(badger2.StorageAskDS), badger2.NewStorageAskDS),
					builder.Override(new(badger2.PayChanDS), badger2.NewPayChanDS),
					builder.Override(new(badger2.PayChanInfoDS), badger2.NewPayChanInfoDs),
					builder.Override(new(badger2.PayChanMsgDs), badger2.NewPayChanMsgDs),
					builder.Override(new(badger2.FundMgrDS), badger2.NewFundMgrDS),
					builder.Override(new(badger2.RetrievalDealsDS), badger2.NewRetrievalDealsDS),
					builder.Override(new(repo.Repo), badger2.NewMigratedBadgerRepo),
				),
			),
		),
			builder.Options(
				// if not server, use badger
				builder.Override(new(badger2.ClientDatastore), badger2.NewClientDatastore),
				builder.Override(new(badger2.ClientBlockstore), badger2.NewClientBlockstore),
				builder.Override(new(badger2.FundMgrDS), badger2.NewFundMgrDS),
				builder.Override(new(badger2.PayChanDS), badger2.NewPayChanDS),
				builder.Override(new(badger2.PayChanInfoDS), badger2.NewPayChanInfoDs),
				builder.Override(new(badger2.PayChanMsgDs), badger2.NewPayChanMsgDs),
				builder.Override(new(badger2.ClientDealsDS), badger2.NewClientDealsDS),
				builder.Override(new(badger2.RetrievalClientDS), badger2.NewRetrievalClientDS),
				builder.Override(new(badger2.ImportClientDS), badger2.NewImportClientDS),
				builder.Override(new(badger2.ClientTransferDS), badger2.NewClientTransferDS),
				builder.Override(new(badger2.ClientOfflineDealsDS), badger2.NewClientOfflineDealStore),
				builder.Override(new(repo.Repo), badger2.NewMigratedBadgerRepo),
				builder.Override(new(repo.ClientOfflineDealRepo), badger2.NewBadgerClientOfflineDealRepo),
			),
		),
	)
}
