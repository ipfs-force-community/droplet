package mysql

import (
	"context"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"gorm.io/gorm"
)

const shardTableName = "shards"

type shard struct {
	Key           string              `gorm:"column:key;primaryKey;type:varchar(128)"`
	URL           string              `gorm:"column:url;type:varchar(256)"`
	TransientPath string              `gorm:"column:transient_path;type:varchar(256)"`
	State         dagstore.ShardState `gorm:"column:state;type:varchar(32)"`
	Lazy          bool                `gorm:"column:lazy"`
	Error         string              `gorm:"column:error;type:varchar(1024)"`
}

func (s *shard) TableName() string {
	return shardTableName
}

type shardRepo struct {
	*gorm.DB
}

func NewShardRepo(db *gorm.DB) repo.IShardRepo {
	return &shardRepo{DB: db}
}

func to(s *shard) *dagstore.PersistedShard {
	return &dagstore.PersistedShard{
		Key:           s.Key,
		URL:           s.URL,
		TransientPath: s.TransientPath,
		State:         s.State,
		Lazy:          s.Lazy,
		Error:         s.Error,
	}
}

func from(s *dagstore.PersistedShard) *shard {
	return &shard{
		Key:           s.Key,
		URL:           s.URL,
		TransientPath: s.TransientPath,
		State:         s.State,
		Lazy:          s.Lazy,
		Error:         s.Error,
	}
}

func (s *shardRepo) CreateShard(ctx context.Context, shard *dagstore.PersistedShard) error {
	return s.DB.WithContext(ctx).Create(from(shard)).Error
}

func (s *shardRepo) SaveShard(ctx context.Context, shard *dagstore.PersistedShard) error {
	return s.DB.WithContext(ctx).Save(from(shard)).Error
}

func (s *shardRepo) GetShard(ctx context.Context, key string) (*dagstore.PersistedShard, error) {
	var shard shard
	if err := s.DB.WithContext(ctx).Take(&shard, "`key` = ?", key).Error; err != nil {
		return nil, err
	}

	return to(&shard), nil
}

func (s *shardRepo) ListShards(ctx context.Context) ([]*dagstore.PersistedShard, error) {
	var shards []*shard
	if err := s.DB.WithContext(ctx).Find(&shards).Error; err != nil {
		return nil, err
	}
	out := make([]*dagstore.PersistedShard, 0, len(shards))
	for _, shard := range shards {
		out = append(out, to(shard))
	}

	return out, nil
}

func (s *shardRepo) HasShard(ctx context.Context, key string) (bool, error) {
	var count int64
	if err := s.DB.Model(&shard{}).WithContext(ctx).Where("`key` = ?", key).
		Count(&count).Error; err != nil {
		return false, nil
	}
	return count > 0, nil
}

func (s *shardRepo) DeleteShard(ctx context.Context, key string) error {
	return s.DB.WithContext(ctx).Where("`key` = ?", key).Delete(&shard{}).Error
}
