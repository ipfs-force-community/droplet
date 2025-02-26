package dagstore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	mh "github.com/multiformats/go-multihash"

	"github.com/ipfs-force-community/droplet/v2/config"
	mock_dagstore "github.com/ipfs-force-community/droplet/v2/dagstore/mocks"
	"github.com/ipfs-force-community/droplet/v2/models/badger"
	carindex "github.com/ipld/go-car/v2/index"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

//go:generate go run github.com/golang/mock/mockgen -destination=./mocks/mock_dagstore_interface.go -package=mock_dagstore  -mock_names Interface=MockDagStoreInterface github.com/filecoin-project/dagstore Interface

// TestWrapperLoadShard verifies that if acquire shard returns a "not found"
// error, the wrapper will attempt to register the shard then reacquire
func TestWrapperLoadShard(t *testing.T) {
	ctx := context.Background()
	pieceCid, err := cid.Parse("bafkqaaa")
	require.NoError(t, err)

	// Create a DAG store wrapper
	dagst, w, err := NewDAGStore(ctx, &config.DAGStoreConfig{
		RootDir:    t.TempDir(),
		GCInterval: config.Duration(1 * time.Millisecond),
	}, mockLotusMount{}, badger.NewBadgerRepo(badger.BadgerDSParams{}))
	require.NoError(t, err)

	defer dagst.Close() //nolint:errcheck

	// Return an error from acquire shard the first time
	acquireShardErr := make(chan error, 1)
	acquireShardErr <- fmt.Errorf("unknown shard: %w", dagstore.ErrShardUnknown)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("re-register shard when not found", func(t *testing.T) {
		dagMock := mock_dagstore.NewMockDagStoreInterface(ctrl)
		w.dagst = dagMock

		dagMock.EXPECT().GetShardInfo(gomock.Any()).Return(dagstore.ShardInfo{}, dagstore.ErrShardUnknown)
		dagMock.EXPECT().RegisterShard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key shard.Key, mnt mount.Mount, out chan dagstore.ShardResult, opts dagstore.RegisterOpts) error {
			out <- dagstore.ShardResult{}
			return nil
		})
		dagMock.EXPECT().GetShardInfo(gomock.Any()).Return(dagstore.ShardInfo{
			ShardState: dagstore.ShardStateAvailable,
		}, nil)
		dagMock.EXPECT().AcquireShard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key shard.Key, out chan dagstore.ShardResult, _ dagstore.AcquireOpts) error {
			out <- dagstore.ShardResult{
				Accessor: getShardAccessor(t),
			}
			return nil
		})

		_, err = w.LoadShard(ctx, pieceCid)
		require.NoError(t, err)
	})

	t.Run("recover shard when shard state error", func(t *testing.T) {
		dagMock := mock_dagstore.NewMockDagStoreInterface(ctrl)
		w.dagst = dagMock

		dagMock.EXPECT().GetShardInfo(gomock.Any()).Return(dagstore.ShardInfo{ShardState: dagstore.ShardStateErrored}, nil)
		dagMock.EXPECT().RecoverShard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key shard.Key, out chan dagstore.ShardResult, _ dagstore.RecoverOpts) error {
			out <- dagstore.ShardResult{}
			return nil
		})
		dagMock.EXPECT().AcquireShard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key shard.Key, out chan dagstore.ShardResult, _ dagstore.AcquireOpts) error {
			out <- dagstore.ShardResult{
				Accessor: getShardAccessor(t),
			}
			return nil
		})

		_, err = w.LoadShard(ctx, pieceCid)
		require.NoError(t, err)

	})
}

// TestWrapperBackground verifies the behaviour of the background go routine
func TestWrapperBackground(t *testing.T) {
	ctx := context.Background()

	// Create a DAG store wrapper
	dagst, w, err := NewDAGStore(ctx, &config.DAGStoreConfig{
		RootDir:      t.TempDir(),
		GCInterval:   config.Duration(1 * time.Millisecond),
		UseTransient: true,
	}, mockLotusMount{}, badger.NewBadgerRepo(badger.BadgerDSParams{}))
	require.NoError(t, err)

	defer dagst.Close() //nolint:errcheck

	// Create a mock DAG store in place of the real DAG store
	mock := &mockDagStore{
		gc:      make(chan struct{}, 1),
		recover: make(chan shard.Key, 1),
		close:   make(chan struct{}, 1),
	}
	w.dagst = mock

	// Start up the wrapper
	err = w.Start(ctx)
	require.NoError(t, err)

	// Expect GC to be called automatically
	tctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	select {
	case <-tctx.Done():
		require.Fail(t, "failed to call GC")
	case <-mock.gc:
	}

	// Expect that when the wrapper is closed it will call close on the
	// DAG store
	err = w.Close()
	require.NoError(t, err)

	tctx, cancel3 := context.WithTimeout(ctx, time.Second)
	defer cancel3()
	select {
	case <-tctx.Done():
		require.Fail(t, "failed to call close")
	case <-mock.close:
	}
}

type mockDagStore struct {
	acquireShardErr chan error
	acquireShardRes dagstore.ShardResult
	register        chan shard.Key

	gc      chan struct{}
	recover chan shard.Key
	destroy chan shard.Key
	close   chan struct{}
}

func (m *mockDagStore) GetIterableIndex(key shard.Key) (carindex.IterableIndex, error) {
	return nil, nil
}

func (m *mockDagStore) ShardsContainingMultihash(ctx context.Context, h mh.Multihash) ([]shard.Key, error) {
	return nil, nil
}

func (m *mockDagStore) DestroyShard(ctx context.Context, key shard.Key, out chan dagstore.ShardResult, _ dagstore.DestroyOpts) error {
	m.destroy <- key
	out <- dagstore.ShardResult{Key: key}
	return nil
}

func (m *mockDagStore) GetShardInfo(k shard.Key) (dagstore.ShardInfo, error) {
	panic("implement me")
}

func (m *mockDagStore) AllShardsInfo() dagstore.AllShardsInfo {
	panic("implement me")
}

func (m *mockDagStore) Start(_ context.Context) error {
	return nil
}

func (m *mockDagStore) RegisterShard(ctx context.Context, key shard.Key, mnt mount.Mount, out chan dagstore.ShardResult, opts dagstore.RegisterOpts) error {
	m.register <- key
	out <- dagstore.ShardResult{Key: key}
	return nil
}

func (m *mockDagStore) AcquireShard(ctx context.Context, key shard.Key, out chan dagstore.ShardResult, _ dagstore.AcquireOpts) error {
	select {
	case err := <-m.acquireShardErr:
		return err
	default:
	}

	out <- m.acquireShardRes
	return nil
}

func (m *mockDagStore) RecoverShard(ctx context.Context, key shard.Key, out chan dagstore.ShardResult, _ dagstore.RecoverOpts) error {
	m.recover <- key
	return nil
}

func (m *mockDagStore) GC(ctx context.Context) (*dagstore.GCResult, error) {
	select {
	case m.gc <- struct{}{}:
	default:
	}

	return nil, nil
}

func (m *mockDagStore) Close() error {
	m.close <- struct{}{}
	return nil
}

type mockLotusMount struct{}

func (m mockLotusMount) Start(ctx context.Context) error {
	return nil
}

func (m mockLotusMount) FetchFromPieceStorage(ctx context.Context, pieceCid cid.Cid) (mount.Reader, error) {
	panic("implement me")
}

func (m mockLotusMount) GetUnpaddedCARSize(ctx context.Context, pieceCid cid.Cid) (uint64, error) {
	panic("implement me")
}

func (m mockLotusMount) IsUnsealed(ctx context.Context, pieceCid cid.Cid) (bool, error) {
	panic("implement me")
}

func getShardAccessor(t *testing.T) *dagstore.ShardAccessor {
	data, err := os.ReadFile("./fixtures/sample-rw-bs-v2.car")
	require.NoError(t, err)
	buff := bytes.NewReader(data)
	reader := &mount.NopCloser{Reader: buff, ReaderAt: buff, Seeker: buff}
	shardAccessor, err := dagstore.NewShardAccessor(reader, nil, nil)
	require.NoError(t, err)
	return shardAccessor
}
