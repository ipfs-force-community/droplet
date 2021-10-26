package StorageAsk

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"golang.org/x/xerrors"
)

type StorageAskCfg struct {
	DbType string
	// to mysql: uri is a connection string,example:
	//  "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
	// to badger: uri is a path
	URI   string
	Debug bool
}

type istorageAskRepo interface {
	GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error)
	SetAsk(miner address.Address, ask *storagemarket.SignedStorageAsk) error
	Close() error
}

type StorageAskRepo struct {
	repo     istorageAskRepo
	provider storagemarket.StorageProviderNode
}

func (repo *StorageAskRepo) Close() error {
	return repo.repo.Close()
}

func (repo *StorageAskRepo) GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	return repo.repo.GetAsk(miner)
}

func (repo *StorageAskRepo) SetAsk(miner address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error {
	minPieceSize := storedask.DefaultMinPieceSize
	maxPieceSize := storedask.DefaultMaxPieceSize

	var seqno uint64

	if s, _ := repo.GetAsk(miner); s != nil {
		seqno = s.Ask.SeqNo
		minPieceSize = s.Ask.MinPieceSize
		maxPieceSize = s.Ask.MaxPieceSize
	}

	ctx := context.TODO()

	_, height, err := repo.provider.GetChainHead(ctx)
	if err != nil {
		return err
	}

	ask := &storagemarket.StorageAsk{
		Price:         price,
		VerifiedPrice: verifiedPrice,
		Timestamp:     height,
		Expiry:        height + duration,
		Miner:         miner,
		SeqNo:         seqno,
		MinPieceSize:  minPieceSize,
		MaxPieceSize:  maxPieceSize,
	}

	for _, option := range options {
		option(ask)
	}

	var signData []byte
	var sig *crypto.Signature

	if sig, err = repo.provider.SignBytes(ctx, miner, signData); err != nil {
		return xerrors.Errorf("Miner:%s sign data failed", miner.String(), err)
	}

	return repo.repo.SetAsk(miner, &storagemarket.SignedStorageAsk{Ask: ask, Signature: sig})
}

func NewStorageAsk(cfg *StorageAskCfg, provider storagemarket.StorageProviderNode) (*StorageAskRepo, error) {
	var iRepo istorageAskRepo
	var err error
	switch cfg.DbType {
	case "mysql":
		if iRepo, err = newMysqlStorageAskRepo(cfg); err != nil {
			return nil, err
		}
		return &StorageAskRepo{repo: iRepo, provider: provider}, nil
	case "badger":
		if iRepo, err = newBadgerStorageAskRepo(cfg); err != nil {
			return nil, err
		}
		return &StorageAskRepo{repo: iRepo, provider: provider}, nil
	default:
		panic(fmt.Sprintf("NewStorageAsk not supported database:%s", cfg.DbType))
	}
}
