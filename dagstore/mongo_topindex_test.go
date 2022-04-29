package dagstore_test

import (
	"context"
	"embed"
	"testing"

	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/filecoin-project/venus-market/v2/dagstore"
	carindex "github.com/ipld/go-car/v2/index"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/strikesecurity/strikememongo"
)

//go:embed fixtures
var res embed.FS

func TestAddMultihashesForShard(t *testing.T) {
	mongoServer, err := strikememongo.StartWithOptions(&strikememongo.Options{MongoVersion: "4.2.1"})
	assert.Nil(t, err)
	defer mongoServer.Stop()
	assert.NotNil(t, mongoServer)

	ctx := context.Background()
	indexSaver, err := dagstore.NewMongoTopIndex(ctx, mongoServer.URI())
	assert.Nil(t, err)
	entries, err := res.ReadDir("fixtures/index")
	assert.Nil(t, err)

	for _, entry := range entries {
		key := shard.KeyFromString(entry.Name())
		{
			f, err := res.Open("fixtures/index/" + entry.Name())
			assert.Nil(t, err)
			index, err := carindex.ReadFrom(f)
			assert.Nil(t, err)
			iterableIdx, _ := index.(carindex.IterableIndex)
			mhIter := &mhIdx{iterableIdx: iterableIdx}
			err = indexSaver.AddMultihashesForShard(ctx, mhIter, key)
			assert.Nil(t, err)
		}

		{
			f, err := res.Open("fixtures/index/" + entry.Name())
			assert.Nil(t, err)
			index, err := carindex.ReadFrom(f)
			assert.Nil(t, err)
			iterableIdx, _ := index.(carindex.IterableIndex)
			err = iterableIdx.ForEach(func(val multihash.Multihash, _ uint64) error {
				keys, err := indexSaver.GetShardsForMultihash(ctx, val)
				assert.Nil(t, err)
				assert.Contains(t, keys, key)
				return nil
			})
			assert.Nil(t, err)
		}
	}
}

// Convenience struct for converting from CAR index.IterableIndex to the
// iterator required by the dag store inverted index.
type mhIdx struct {
	iterableIdx carindex.IterableIndex
}

var _ index.MultihashIterator = (*mhIdx)(nil)

func (it *mhIdx) ForEach(fn func(mh multihash.Multihash) error) error {
	return it.iterableIdx.ForEach(func(mh multihash.Multihash, _ uint64) error {
		return fn(mh)
	})
}
