package mysql

import (
	"time"

	"golang.org/x/xerrors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/interfaces"
)

type MysqlRepo struct {
	*gorm.DB
}

func (r MysqlRepo) GetDb() *gorm.DB {
	return r.DB
}

func (r MysqlRepo) FundRepo() interfaces.FundRepo {
	return NewFundedAddressStateRepo(r.GetDb())
}

func (r MysqlRepo) MinerParamsRepo() interfaces.MinerParamsRepo {
	return NewMinerParamsRepo(r.GetDb())
}

func (r MysqlRepo) MinerDealRepo() interfaces.MinerDealRepo {
	return NewMinerDealRepo(r.GetDb())
}

func (r MysqlRepo) PaychMsgInfoRepo() interfaces.PaychMsgInfoRepo {
	return NewMsgInfoRepo(r.GetDb())
}

func (r MysqlRepo) PaychChannelInfo() interfaces.PaychChannelInfoRepo {
	return NewChannelInfoRepo(r.GetDb())
}

func InitMysql(cfg *config.Mysql) (interfaces.Repo, error) {
	db, err := gorm.Open(mysql.Open(cfg.ConnectionString))

	if err != nil {
		return nil, xerrors.Errorf("[db connection failed] Database name: %s %w", cfg.ConnectionString, err)
	}

	db.Set("gorm:table_options", "CHARSET=utf8mb4")
	if cfg.Debug {
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConn)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConn)
	d, err := time.ParseDuration(cfg.ConnMaxLifeTime)
	if err != nil {
		return nil, err
	}
	sqlDB.SetConnMaxLifetime(d)

	r := &MysqlRepo{DB: db}

	return r, r.AutoMigrate(minerParams{}, fundedAddressState{}, minerDeal{}, channelInfo{}, msgInfo{})
}
