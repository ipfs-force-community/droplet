package mysql

import (
	"database/sql/driver"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/libp2p/go-libp2p-core/peer"

	mtypes "github.com/filecoin-project/venus-messager/types"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/repo"
)

type MysqlRepo struct {
	*gorm.DB
}

var _ repo.Repo = MysqlRepo{}

func (r MysqlRepo) GetDb() *gorm.DB {
	return r.DB
}

func (r MysqlRepo) FundRepo() repo.FundRepo {
	return NewFundedAddressStateRepo(r.GetDb())
}

func (r MysqlRepo) StorageDealRepo() repo.StorageDealRepo {
	return NewStorageDealRepo(r.GetDb())
}

func (r MysqlRepo) PaychMsgInfoRepo() repo.PaychMsgInfoRepo {
	return NewMsgInfoRepo(r.GetDb())
}

func (r MysqlRepo) PaychChannelInfoRepo() repo.PaychChannelInfoRepo {
	return NewChannelInfoRepo(r.GetDb())
}

func (r MysqlRepo) StorageAskRepo() repo.IStorageAskRepo {
	return NewStorageAskRepo(r.GetDb())
}

func (r MysqlRepo) RetrievalAskRepo() repo.IRetrievalAskRepo {
	return NewRetrievalAskRepo(r.GetDb())
}

func (r MysqlRepo) CidInfoRepo() repo.ICidInfoRepo {
	return NewMysqlCidInfoRepo(r.GetDb())
}

func (r MysqlRepo) RetrievalDealRepo() repo.IRetrievalDealRepo {
	return NewRetrievalDealRepo(r.GetDb())
}

func (r MysqlRepo) Close() error {
	db, err := r.DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func InitMysql(cfg *config.Mysql) (repo.Repo, error) {
	gorm.ErrRecordNotFound = repo.ErrNotFound

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

	return r, r.AutoMigrate(modelRetrievalAsk{}, cidInfo{}, storageAsk{}, fundedAddressState{}, storageDeal{}, channelInfo{}, msgInfo{})
}

func parseCid(str string) (cid.Cid, error) {
	if len(str) == 0 {
		return cid.Undef, nil
	}

	return cid.Parse(str)
}

func decodeCid(c cid.Cid) string {
	if c == cid.Undef {
		return ""
	}
	return c.String()
}

func parseCidPtr(str string) (*cid.Cid, error) {
	if len(str) == 0 {
		return nil, nil
	}
	c, err := cid.Parse(str)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func decodeCidPtr(c *cid.Cid) string {
	if c == nil {
		return ""
	}
	return c.String()
}

func convertBigInt(v big.Int) mtypes.Int {
	if v.Nil() {
		return mtypes.Zero()
	}
	return mtypes.NewFromGo(v.Int)
}

func decodePeerId(str string) (peer.ID, error) {
	return peer.Decode(str)
}

type Address address.Address

var UndefAddress = Address{}

func (a *Address) Scan(value interface{}) error {
	val, ok := value.([]byte)
	if !ok {
		return xerrors.New("address should be a `[]byte`")
	}
	v := string(val)
	if v == address.UndefAddressString {
		*a = UndefAddress
		return nil
	}
	addr, err := address.NewFromString(address.MainnetPrefix + v)
	if err != nil {
		return err
	}
	*a = toAddress(addr)

	return nil
}

func (a Address) Value() (driver.Value, error) {
	if a == UndefAddress {
		return []byte(address.UndefAddressString), nil
	}
	// Remove the prefix identifying the network type，eg. change `f01000` to `01000`
	return a.String()[1:], nil
}

func (a Address) String() string {
	return a.addr().String()
}

func (a Address) addr() address.Address {
	return address.Address(a)
}

func (a *Address) addrPtr() *address.Address {
	if a == nil {
		return nil
	}
	addr := address.Address(*a)
	return &addr
}

func toAddress(addr address.Address) Address {
	return Address(addr)
}

func toAddressPtr(addrPtr *address.Address) *Address {
	if addrPtr == nil {
		return nil
	}
	addr := toAddress(*addrPtr)
	return &addr
}

func cutPrefix(addr address.Address) string {
	if addr == address.Undef {
		return address.UndefAddressString
	}
	// Remove the prefix identifying the network type，eg. change `f01000` to `01000`
	return addr.String()[1:]
}
