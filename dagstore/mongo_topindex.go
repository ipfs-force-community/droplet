package dagstore

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/multiformats/go-multihash"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ index.Inverted = (*MongoTopIndex)(nil)

type TopIndex struct {
	Id     string   `bson:"_id"`
	Pieces []string `bson:"pieces"`
}

type MongoTopIndex struct {
	indexCol *mongo.Collection
}

func NewMongoTopIndex(ctx context.Context, url string) (index.Inverted, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(url))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongog databse (%s) %w", url, err)
	}
	return &MongoTopIndex{
		indexCol: client.Database("market_index").Collection("top_index"),
	}, nil
}

func (mongoTopIndex *MongoTopIndex) AddMultihashesForShard(ctx context.Context, mhIter index.MultihashIterator, s shard.Key) error {
	var models []mongo.WriteModel
	if err := mhIter.ForEach(func(mh multihash.Multihash) error {
		updateOne := mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": mh.HexString()}).SetUpdate(bson.M{
			"$addToSet": bson.M{
				"pieces": s.String(),
			},
		}).SetUpsert(true)
		models = append(models, updateOne)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to build mongo insert model: %w", err)
	}
	res, err := mongoTopIndex.indexCol.BulkWrite(ctx, models)
	log.Infow("bulk write shard to mongo", "insert", res.InsertedCount, "update", res.ModifiedCount, "upsert", res.UpsertedCount)
	return err
}

// GetShardsForMultihash returns keys for all the shards that has the given multihash.
func (mongoTopIndex *MongoTopIndex) GetShardsForMultihash(ctx context.Context, h multihash.Multihash) ([]shard.Key, error) {
	sigleResult := mongoTopIndex.indexCol.FindOne(ctx, bson.M{"_id": h.HexString()})
	if sigleResult.Err() != nil {
		return nil, sigleResult.Err()
	}
	var tipIndex TopIndex
	err := sigleResult.Decode(&tipIndex)
	if err != nil {
		return nil, err
	}
	var shardKeys []shard.Key
	for _, r := range tipIndex.Pieces {
		shardKeys = append(shardKeys, shard.KeyFromString(r))
	}
	return shardKeys, nil
}
