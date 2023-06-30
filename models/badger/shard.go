package badger

import (
	"context"

	"github.com/filecoin-project/dagstore"
)

// dagstore already implements ShardRepo, so we don't need to it again.
// https://github.com/ipfs-force-community/dagstore/blob/master/shard_repo.go#L27
type Shard struct{}

func NewShardRepo() *Shard {
	return &Shard{}
}

func (s *Shard) CreateShard(ctx context.Context, shard *dagstore.PersistedShard) error {
	panic("implement me")
}

func (s *Shard) SaveShard(ctx context.Context, shard *dagstore.PersistedShard) error {
	panic("implement me")
}
func (s *Shard) GetShard(ctx context.Context, key string) (*dagstore.PersistedShard, error) {
	panic("implement me")
}
func (s *Shard) ListShards(ctx context.Context) ([]*dagstore.PersistedShard, error) {
	panic("implement me")
}
func (s *Shard) HasShard(ctx context.Context, key string) (bool, error) {
	panic("implement me")
}
func (s *Shard) DeleteShard(ctx context.Context, key string) error {
	panic("implement me")
}
