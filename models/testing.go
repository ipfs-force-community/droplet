package models

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/mysql"
	"github.com/filecoin-project/venus-market/v2/utils/test_helper"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models/repo"
)

func MysqlDB(t *testing.T) repo.Repo {
	connSQL := test_helper.Mysql(t)
	repo, err := mysql.InitMysql(&config.Mysql{
		ConnectionString: connSQL,
		MaxOpenConn:      10,
		MaxIdleConn:      10,
		ConnMaxLifeTime:  "1m",
		Debug:            true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func BadgerDB(t *testing.T) *badger.Datastore {
	opts := &badger.DefaultOptions
	opts.InMemory = true
	db, err := badger.NewDatastore("", opts)
	assert.Nil(t, err)
	return db
}

func randAddress(t *testing.T) address.Address {
	addr, err := address.NewActorAddress([]byte(uuid.New().String()))
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

func randCid(t *testing.T) cid.Cid {
	totalLen := 62
	b := bytes.Buffer{}
	data := []byte("bafy2bzacedfra7y3yb5feuxm3iizqubo3jufhrwfw6yy74")
	_, err := b.Write(data)
	assert.Nil(t, err)
	for i := 0; i < totalLen-len(data); i++ {
		idx := rand.Intn(len(data))
		assert.Nil(t, b.WriteByte(data[idx]))
	}
	id, err := cid.Decode(b.String())
	assert.Nil(t, err)
	return id
}

func RandStorageAsk(t *testing.T) *storagemarket.StorageAsk {
	return &storagemarket.StorageAsk{
		Price:         abi.NewTokenAmount(10),
		VerifiedPrice: abi.NewTokenAmount(100),
		MinPieceSize:  1024,
		MaxPieceSize:  1024,
		Miner:         randAddress(t),
		Timestamp:     abi.ChainEpoch(10),
		Expiry:        abi.ChainEpoch(10),
		SeqNo:         0,
	}
}
