package repo

import (
	"gorm.io/gorm"
)

const (
	NotDeleted = -1
	Deleted    = 1
)

type Repo interface {
	GetDb() *gorm.DB
	Transaction(func(txRepo TxRepo) error) error
	DbClose() error
	AutoMigrate() error
}

type TxRepo interface {
}
