package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	dagstore2 "github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-jsonrpc"
	marketapi "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/dagstore"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipld/go-car/v2"
	carindex "github.com/ipld/go-car/v2/index"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

const indexSuffix = ".full.idx"

var (
	mongoURLFlag = &cli.StringFlag{
		Name:     "mongo-url",
		Usage:    "mongo url, use for store topIndex",
		Required: true,
	}
	mysqlURLFlag = &cli.StringFlag{
		Name:     "mysql-url",
		Usage:    "mysql url, use for store shard",
		Required: true,
	}
	indexDirFlag = &cli.StringFlag{
		Name:     "index-dir",
		Usage:    "The directory where the index is stored",
		Required: true,
	}
	carDirFlag = &cli.StringFlag{
		Name:     "car-dir",
		Usage:    "directory for car files",
		Required: true,
	}
	dropletURLFlag = &cli.StringFlag{
		Name:     "droplet-url",
		Usage:    "droplet url",
		Required: true,
	}
	dropletTokenFlag = &cli.StringFlag{
		Name:     "droplet-token",
		Usage:    "droplet token",
		Required: true,
	}
)

func main() {
	app := cli.App{
		Name:  "index-tool",
		Usage: "Used to generate indexes and migrate indexes",
		Commands: []*cli.Command{
			generateIndexCmd,
			migrateIndexCmd,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var generateIndexCmd = &cli.Command{
	Name:  "gen-index",
	Usage: "generate car index",
	Flags: []cli.Flag{
		mongoURLFlag,
		mysqlURLFlag,
		indexDirFlag,
		carDirFlag,
		dropletTokenFlag,
		dropletURLFlag,
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		carDir := cctx.String("car-dir")
		indexDir := cctx.String(indexDirFlag.Name)
		p, err := paramsFromContext(cctx)
		if err != nil {
			return err
		}

		fmt.Println("car dir:", carDir, "index dir:", indexDir)

		return generateIndex(ctx, carDir, indexDir, p)
	},
}

type params struct {
	api          marketapi.IMarket
	close        jsonrpc.ClientCloser
	topIndexRepo *dagstore.MongoTopIndex
	shardRepo    repo.IShardRepo
	pieces       map[string]struct{}
}

func paramsFromContext(cctx *cli.Context) (*params, error) {
	ctx := cctx.Context
	mongoURL := cctx.String(mongoURLFlag.Name)
	mysqlURL := cctx.String(mysqlURLFlag.Name)
	url := cctx.String(dropletURLFlag.Name)
	token := cctx.String(dropletTokenFlag.Name)

	fmt.Println("mongo url:", mongoURL)
	fmt.Println("mysql url:", mysqlURL)
	fmt.Println("droplet url:", url, "token:", token)

	api, close, err := marketapi.DialIMarketRPC(ctx, url, token, nil)
	if err != nil {
		return nil, err
	}
	defer close()

	activeDeal := storagemarket.StorageDealActive
	deals, err := api.MarketListIncompleteDeals(ctx, &market.StorageDealQueryParams{State: &activeDeal})
	if err != nil {
		return nil, fmt.Errorf("list deal failed: %v", err)
	}
	pieces := make(map[string]struct{}, len(deals))
	for _, deal := range deals {
		pieces[deal.Proposal.PieceCID.String()] = struct{}{}
	}
	fmt.Printf("had %d active deals, had %d piece\n", len(deals), len(pieces))

	topIndexRepo, err := dagstore.NewMongoTopIndex(ctx, mongoURL)
	if err != nil {
		return nil, fmt.Errorf("connect to mongo failed: %v", err)
	}

	cfg := config.DefaultMarketConfig
	cfg.Mysql.ConnectionString = mysqlURL
	repo, err := mysql.InitMysql(&cfg.Mysql)
	if err != nil {
		return nil, fmt.Errorf("connect to mysql failed: %v", err)
	}

	return &params{
		api:          api,
		close:        close,
		topIndexRepo: topIndexRepo,
		shardRepo:    repo.ShardRepo(),
		pieces:       pieces,
	}, nil
}

func generateIndex(ctx context.Context, carDir string, indexDir string, p *params) error {
	for piece := range p.pieces {
		has, err := hasIndex(ctx, piece, indexDir)
		if err != nil {
			return err
		}
		if has {
			continue
		}

		f, err := os.Open(filepath.Join(carDir, piece))
		if err != nil {
			fmt.Println(err)
			continue
		}
		defer f.Close() //nolint

		idx, err := car.ReadOrGenerateIndex(f, car.ZeroLengthSectionAsEOF(true), car.StoreIdentityCIDs(true))
		if err == nil {
			fmt.Printf("generate index success: %s\n", piece)

			if err := saveIndex(idx, indexDir, piece); err != nil {
				return fmt.Errorf("save index failed, piece: %s, error: %v", piece, err)
			}
			if err := saveTopIndexToMongo(ctx, piece, idx, p.topIndexRepo); err != nil {
				return fmt.Errorf("save top index to mongo failed, piece: %s, error: %v", piece, err)
			}
			if err := saveShardToMysql(ctx, piece, p.shardRepo); err != nil {
				return fmt.Errorf("save shard to mysql failed, piece: %s, error: %vs", piece, err)
			}
		} else {
			fmt.Printf("generate index failed, piece: %s, error: %v\n", piece, err)
		}
	}

	return nil
}

func hasIndex(ctx context.Context, piece string, indexDir string) (bool, error) {
	_, err := os.Stat(filepath.Join(indexDir, piece+indexSuffix))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func saveIndex(idx carindex.Index, dir string, piece string) error {
	f, err := os.Create(filepath.Join(dir, piece+indexSuffix))
	if err != nil {
		return err
	}
	defer f.Close() //nolint

	_, err = carindex.WriteTo(idx, f)
	return err
}

func saveTopIndexToMongo(ctx context.Context, key string, idx carindex.Index, indexRepo *dagstore.MongoTopIndex) error {
	// add all cids in the shard to the inverted (cid -> []Shard Keys) index.
	iterableIdx, ok := idx.(carindex.IterableIndex)
	if ok {
		if err := indexRepo.AddMultihashesForShard(ctx, &mhIdx{iterableIdx}, shard.KeyFromString(key)); err != nil {
			return err
		}
	}

	return nil
}

func saveShardToMysql(ctx context.Context, piece string, shardRepo repo.IShardRepo) error {
	shard := dagstore2.PersistedShard{
		Key:   piece,
		URL:   fmt.Sprintf("market://%s", piece),
		State: dagstore2.ShardStateAvailable,
		Lazy:  true,
		Error: "",
	}

	return shardRepo.CreateShard(ctx, &shard)
}

var migrateIndexCmd = &cli.Command{
	Name:  "migrate-index",
	Usage: "migrate top index to MongoDB and migrate shard state to mysql",
	Flags: []cli.Flag{
		mongoURLFlag,
		mysqlURLFlag,
		indexDirFlag,
		dropletURLFlag,
		dropletTokenFlag,
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		indexDir := cctx.String(indexDirFlag.Name)
		p, err := paramsFromContext(cctx)
		if err != nil {
			return err
		}

		fmt.Println("index dir:", indexDir)

		return migrateIndex(ctx, indexDir, p)
	},
}

func migrateIndex(ctx context.Context, indexDir string, p *params) error {
	return filepath.Walk(indexDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := info.Name()
		if !strings.HasSuffix(name, indexSuffix) {
			return nil
		}
		piece := name[:len(name)-len(indexSuffix)]
		if _, ok := p.pieces[piece]; !ok {
			return nil
		}
		indexPath := filepath.Join(indexDir, name)
		has, err := p.shardRepo.HasShard(ctx, piece)
		if err != nil {
			return err
		}
		if has {
			return nil
		}

		f, err := os.Open(indexPath)
		if err != nil {
			return err
		}
		defer f.Close() //nolint
		idx, err := carindex.ReadFrom(f)
		if err != nil {
			return err
		}

		if err := saveTopIndexToMongo(ctx, piece, idx, p.topIndexRepo); err != nil {
			return fmt.Errorf("save top index to mongo failed, piece: %s, error: %v", piece, err)
		}
		if err := saveShardToMysql(ctx, piece, p.shardRepo); err != nil {
			return fmt.Errorf("save shard to mysql failed, piece: %s, error: %vs", piece, err)
		}
		fmt.Printf("migrate %s success\n", piece)

		return nil
	})
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
