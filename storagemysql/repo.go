package storagemysql

import "gorm.io/gorm"

type TxRepo interface {
}

type Repo interface {
	GetDb() *gorm.DB
	Transaction(func(txRepo TxRepo) error) error
	DbClose() error
	AutoMigrate() error
}

type repo struct {
	*gorm.DB
}

func (r repo) AutoMigrate() error {
	return r.DB.AutoMigrate()
}

func (r repo) GetDb() *gorm.DB {
	return r.DB
}

func (r repo) DbClose() error {
	return nil
}

func (r repo) Transaction(cb func(txRepo TxRepo) error) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		txRepo := &txRepo{DB: tx}
		return cb(txRepo)
	})
}

type txRepo struct {
	*gorm.DB
}

var _ Repo = (*repo)(nil)
var _ TxRepo = (*txRepo)(nil)
