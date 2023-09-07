package client

import (
	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type Repo interface {
	ClientPieceInfoRepo() ClientPieceInfoRepo
}

func NewMysqlRepo(cfg *config.Mysql) (Repo, error) {
	db, err := mysql.InitMysql(cfg)
	if err != nil {
		return nil, err
	}
	r := MysqlRepo{DB: db}

	return r, r.Migrate()
}

type MysqlRepo struct {
	*gorm.DB
}

func (r MysqlRepo) GetDb() *gorm.DB {
	return r.DB
}

func (r MysqlRepo) Close() error {
	db, err := r.DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func (r MysqlRepo) Migrate() error {
	return r.AutoMigrate(pieceInfo{})
}

func (r MysqlRepo) ClientPieceInfoRepo() ClientPieceInfoRepo {
	return NewClientPieceInfoRepo(r.GetDb())
}

type ClientPieceInfoRepo interface {
	SavePieceInfo(pi *types.ClientPieceInfo) error
	GetPieceInfo(pieceCID cid.Cid) (*types.ClientPieceInfo, error)
	ListPieceInfo() ([]*types.ClientPieceInfo, error)
}
