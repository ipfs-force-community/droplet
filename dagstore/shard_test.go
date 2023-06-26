package dagstore

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var columns = []string{"key", "url", "transient_path", "state", "lazy", "error"}

func setup(t *testing.T) (shardRepo, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT VERSION()").WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(""))

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: sqlDB,
	}))
	assert.NoError(t, err)

	return shardRepo{DB: gormDB}, mock, func() {
		_ = sqlDB.Close()
	}
}

func TestGet(t *testing.T) {
	r, mock, close := setup(t)
	defer close()

	var shard internalShard
	testutil.Provide(t, &shard)
	data, err := json.Marshal(shard)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `shards` WHERE `key` = ? LIMIT 1")).WithArgs(shard.Key).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(shard.Key, shard.URL, shard.TransientPath, shard.State, shard.Lazy, shard.Error))

	res, err := r.Get(context.Background(), ds.NewKey(shard.Key))
	assert.NoError(t, err)
	assert.Equal(t, data, res)
}

func TestHas(t *testing.T) {
	r, mock, close := setup(t)
	defer close()

	var shard internalShard
	testutil.Provide(t, &shard)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `shards` WHERE `key` = ?")).WithArgs(shard.Key).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	res, err := r.Has(context.Background(), ds.NewKey(shard.Key))
	assert.NoError(t, err)
	assert.True(t, res)
}

func TestQuery(t *testing.T) {
	r, mock, close := setup(t)
	defer close()

	var shard internalShard
	testutil.Provide(t, &shard)
	data, err := json.Marshal(shard)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `shards`")).WillReturnRows(sqlmock.NewRows(columns).
		AddRow(shard.Key, shard.URL, shard.TransientPath, shard.State, shard.Lazy, shard.Error))

	res, err := r.Query(context.Background(), query.Query{})
	assert.NoError(t, err)
	e, err := res.Rest()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(e))
	assert.Equal(t, data, e[0].Value)
}

func TestPut(t *testing.T) {
	r, mock, close := setup(t)
	defer close()

	var shard internalShard
	testutil.Provide(t, &shard)
	data, err := json.Marshal(shard)
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `shards` SET `url`=?,`transient_path`=?,`state`=?,`lazy`=?,`error`=? WHERE `key` = ?")).
		WithArgs(shard.URL, shard.TransientPath, shard.State, shard.Lazy, shard.Error, shard.Key).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = r.Put(context.Background(), ds.NewKey(shard.Key), data)
	assert.NoError(t, err)
}

func TestDelete(t *testing.T) {
	r, mock, close := setup(t)
	defer close()

	var shard internalShard
	testutil.Provide(t, &shard)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `shards` WHERE `key` = ?")).WithArgs(shard.Key).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.Delete(context.Background(), ds.NewKey(shard.Key))
	assert.NoError(t, err)
}
