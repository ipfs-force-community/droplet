package dagstore

import (
	"context"
	"encoding/json"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/jbenet/goprocess"
	"gorm.io/gorm"
)

const notSupport = "not support"

const shardTableName = "shards"

type internalShard struct {
	Key           string              `gorm:"column:key;primaryKey;type:varchar(128)" json:"k"`
	URL           string              `gorm:"column:url;type:varchar(256)" json:"u"`
	TransientPath string              `gorm:"column:transient_path;type:varchar(256)" json:"t"`
	State         dagstore.ShardState `gorm:"column:state;type:varchar(32)" json:"s"`
	Lazy          bool                `gorm:"column:lazy" json:"l"`
	Error         string              `gorm:"column:error;type:varchar(256)" json:"e"`
}

func (s *internalShard) TableName() string {
	return shardTableName
}

type shardRepo struct {
	*gorm.DB
}

func newShardRepo(cfg *config.Mysql) (*shardRepo, error) {
	db, err := mysql.InitMysql(cfg)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(internalShard{}); err != nil {
		return nil, err
	}

	return &shardRepo{DB: db}, nil
}

var _ ds.Datastore = &shardRepo{}

func (s *shardRepo) Get(ctx context.Context, key ds.Key) (value []byte, err error) {
	var shard internalShard
	if err := s.DB.WithContext(ctx).Take(&shard, "`key` = ?", key.BaseNamespace()).Error; err != nil {
		return nil, err
	}

	return json.Marshal(shard)
}

func (s *shardRepo) Has(ctx context.Context, key ds.Key) (exists bool, err error) {
	var count int64
	if err := s.DB.Model(&internalShard{}).WithContext(ctx).Where("`key` = ?", key.BaseNamespace()).
		Count(&count).Error; err != nil {
		return false, nil
	}
	return count > 0, nil
}

func (s *shardRepo) GetSize(ctx context.Context, key ds.Key) (size int, err error) {
	panic(notSupport)
}

func (s *shardRepo) Query(ctx context.Context, q query.Query) (query.Results, error) {
	var shards []*internalShard
	if err := s.DB.WithContext(ctx).Find(&shards).Error; err != nil {
		return nil, err
	}

	return newResults(q, shards), nil
}

func (s *shardRepo) Put(ctx context.Context, key ds.Key, value []byte) error {
	shard := new(internalShard)
	if err := json.Unmarshal(value, shard); err != nil {
		return err
	}

	return s.DB.WithContext(ctx).Save(shard).Error
}

func (s *shardRepo) Delete(ctx context.Context, key ds.Key) error {
	return s.DB.WithContext(ctx).Where("`key` = ?", key.BaseNamespace()).Delete(&internalShard{}).Error
}

func (s *shardRepo) Sync(ctx context.Context, prefix ds.Key) error {
	return nil
}

func (s *shardRepo) Close() error {
	return nil
}

/////////// results ///////////

type results struct {
	q      query.Query
	shards []*internalShard
	// record sended shard
	idx int
}

var _ query.Results = &results{}

func newResults(q query.Query, shards []*internalShard) *results {
	r := &results{
		q:      q,
		shards: shards,
	}

	return r
}

func (r *results) Query() query.Query {
	return r.q
}

func (r *results) Next() <-chan query.Result {
	out := make(chan query.Result, 1)
	go func() {
		for _, shard := range r.shards {
			out <- toResult(shard)
		}

		close(out)
	}()

	return out
}

func (r *results) NextSync() (query.Result, bool) {
	if r.idx < len(r.shards) {
		shard := r.shards[r.idx]
		r.idx++

		return toResult(shard), true
	}

	return query.Result{}, false
}

func (r *results) Rest() ([]query.Entry, error) {
	res := make([]query.Entry, 0, len(r.shards))
	for _, shard := range r.shards {
		e, err := toEntry(shard)
		if err != nil {
			return nil, err
		}
		res = append(res, e)
	}

	return res, nil
}

func (r *results) Close() error {
	return nil
}

func (r *results) Process() goprocess.Process {
	panic(notSupport)
}

func toResult(s *internalShard) query.Result {
	e, err := toEntry(s)
	return query.Result{
		Entry: e,
		Error: err,
	}
}

func toEntry(s *internalShard) (query.Entry, error) {
	value, err := json.Marshal(s)

	return query.Entry{
		Key:   dagstore.StoreNamespace.ChildString(s.Key).String(),
		Value: value,
	}, err
}
