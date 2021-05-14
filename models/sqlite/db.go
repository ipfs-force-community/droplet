package sqlite

import (
	"golang.org/x/xerrors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
)

type SqlLiteRepo struct {
	*gorm.DB
}

func (d SqlLiteRepo) AutoMigrate() error {
	/*err := d.GetDb().AutoMigrate(sqliteMessage{})
	if err != nil {
		return err
	}*/

	return nil
}

func (d SqlLiteRepo) GetDb() *gorm.DB {
	return d.DB
}

func (d SqlLiteRepo) Transaction(cb func(txRepo repo.TxRepo) error) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		txRepo := &TxSqlliteRepo{tx}
		return cb(txRepo)
	})
}

var _ repo.TxRepo = (*TxSqlliteRepo)(nil)

type TxSqlliteRepo struct {
	*gorm.DB
}

func (d SqlLiteRepo) DbClose() error {
	// todo: if '*gorm.DB' need to dispose?
	return nil
}

func OpenSqlite(cfg *config.SqliteConfig) (repo.Repo, error) {
	//cache=shared&_journal_mode=wal&sync=normal
	//cache=shared&sync=full
	db, err := gorm.Open(sqlite.Open(cfg.Path+"?cache=shared&_journal_mode=wal&sync=normal"), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info), // 日志配置
	})
	if err != nil {
		return nil, xerrors.Errorf("fail to connect sqlite: %s %w", cfg.Path, err)
	}
	db.Set("gorm:table_options", "CHARSET=utf8mb4")

	if cfg.Debug {
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	return &SqlLiteRepo{
		db,
	}, nil
}
