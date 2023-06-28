package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/assert"
)

var columns = []string{"key", "url", "transient_path", "state", "lazy", "error"}

func TestSaveShard(t *testing.T) {
	r, mock, db := setup(t)

	var shard dagstore.PersistedShard
	testutil.Provide(t, &shard)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `shards` SET `url`=?,`transient_path`=?,`state`=?,`lazy`=?,`error`=? WHERE `key` = ?")).
		WithArgs(shard.URL, shard.TransientPath, shard.State, shard.Lazy, shard.Error, shard.Key).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.ShardRepo().SaveShard(context.Background(), &shard)
	assert.NoError(t, err)

	_ = closeDB(mock, db)
}

func TestGetShard(t *testing.T) {
	r, mock, db := setup(t)

	var shard dagstore.PersistedShard
	testutil.Provide(t, &shard)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `shards` WHERE `key` = ? LIMIT 1")).WithArgs(shard.Key).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(shard.Key, shard.URL, shard.TransientPath, shard.State, shard.Lazy, shard.Error))

	res, err := r.ShardRepo().GetShard(context.Background(), shard.Key)
	assert.NoError(t, err)
	assert.Equal(t, &shard, res)

	_ = closeDB(mock, db)
}

func TestListShards(t *testing.T) {
	r, mock, db := setup(t)

	var shard dagstore.PersistedShard
	testutil.Provide(t, &shard)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `shards`")).WillReturnRows(sqlmock.NewRows(columns).
		AddRow(shard.Key, shard.URL, shard.TransientPath, shard.State, shard.Lazy, shard.Error))

	res, err := r.ShardRepo().ListShards(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, &shard, res[0])

	_ = closeDB(mock, db)
}

func TestHasShard(t *testing.T) {
	r, mock, db := setup(t)

	var shard dagstore.PersistedShard
	testutil.Provide(t, &shard)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `shards` WHERE `key` = ?")).WithArgs(shard.Key).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	res, err := r.ShardRepo().HasShard(context.Background(), shard.Key)
	assert.NoError(t, err)
	assert.True(t, res)

	_ = closeDB(mock, db)
}

func TestDeleteShard(t *testing.T) {
	r, mock, db := setup(t)

	var shard dagstore.PersistedShard
	testutil.Provide(t, &shard)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `shards` WHERE `key` = ?")).WithArgs(shard.Key).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := r.ShardRepo().DeleteShard(context.Background(), shard.Key)
	assert.NoError(t, err)

	_ = closeDB(mock, db)
}
