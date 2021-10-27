package models

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/models/mysql"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models/itf"
)

func mysqlDB(t *testing.T) itf.Repo {
	repo, err := mysql.InitMysql(&config.Mysql{
		ConnectionString: "root:Root1234@(127.0.0.1:3306)/venus_market_test?parseTime=true&loc=Local",
		MaxOpenConn:      10,
		MaxIdleConn:      10,
		ConnMaxLifeTime:  "1m",
		Debug:            false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func badgerDB(t *testing.T, path string) *badger.Datastore {
	db, err := badger.NewDatastore(path, &badger.DefaultOptions)
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
