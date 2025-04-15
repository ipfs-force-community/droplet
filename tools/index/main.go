package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	dagstore2 "github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/filecoin-project/dagstore/throttle"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-jsonrpc"
	marketapi "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/dagstore"
	"github.com/ipfs-force-community/droplet/v2/models/mysql"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/v2"
	carindex "github.com/ipld/go-car/v2/index"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

const indexSuffix = ".full.idx"

var (
	mongoURLFlag = &cli.StringFlag{
		Name:  "mongo-url",
		Usage: "mongo url, use for store topIndex",
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
		Name:  "droplet-url",
		Usage: "droplet url",
		Value: "/ip4/127.0.0.1/tcp/41264",
	}
	dropletTokenFlag = &cli.StringFlag{
		Name:     "droplet-token",
		Usage:    "droplet token",
		Required: true,
	}
	startFlag = &cli.StringFlag{
		Name:  "start",
		Usage: "The index will only be created when the deal creation time is greater than 'start', eg. 2023-07-26",
	}
	endFlag = &cli.StringFlag{
		Name:  "end",
		Usage: "The index will only be created when the deal creation time is less than end 'end', eg. 2023-07-27",
	}
	concurrencyFlag = &cli.IntFlag{
		Name:  "concurrency",
		Usage: "Concurrent number of indexes generated",
		Value: 1,
	}
	minersAddrFlag = &cli.StringFlag{
		Name:  "miner-addr",
		Usage: "miner address, eg --miner-addr t010001 or --miner-addr t010001,t010002",
	}
)

func main() {
	app := cli.App{
		Name:  "index-tool",
		Usage: "Used to generate indexes and migrate indexes",
		Commands: []*cli.Command{
			generateIndexCmd,
			migrateIndexCmd,
			indexInfoCmd,
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
		startFlag,
		endFlag,
		minersAddrFlag,
		concurrencyFlag,
		&cli.BoolFlag{
			Name: "once",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		carDir := cctx.String(carDirFlag.Name)
		indexDir := cctx.String(indexDirFlag.Name)
		p, err := paramsFromContext(cctx, nil)
		if err != nil {
			return err
		}
		p.concurrency = max(cctx.Int(concurrencyFlag.Name), 1)

		fmt.Println("car dir:", carDir, "index dir:", indexDir)

		if cctx.Bool("once") {
			return generateIndex(ctx, carDir, indexDir, p)
		}

		ticker := time.NewTicker(time.Hour * 3)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Println("start generate index")
				start := time.Now()
				p, err := paramsFromContext(cctx, p.topIndexRepo)
				if err != nil {
					fmt.Println("params from context failed: %v", err)
					continue
				}
				p.concurrency = max(cctx.Int(concurrencyFlag.Name), 1)

				if err := generateIndex(ctx, carDir, indexDir, p); err != nil {
					fmt.Printf("generate index failed: %v\n", err)
					continue
				}
				fmt.Println("generate index done, took: ", time.Since(start))
			case <-ctx.Done():
				fmt.Println("context done")
				return nil
			}
		}
	},
}

type pieceInfo struct {
	piece       string
	payloadSize uint64
	pieceSize   uint64
}

type params struct {
	api          marketapi.IMarket
	close        jsonrpc.ClientCloser
	topIndexRepo *dagstore.MongoTopIndex
	shardRepo    repo.IShardRepo
	pieces       map[string]struct{}
	pieceInfos   []*pieceInfo
	concurrency  int
}

func paramsFromContext(cctx *cli.Context, topIndexRepo *dagstore.MongoTopIndex) (*params, error) {
	ctx := cctx.Context
	mongoURL := cctx.String(mongoURLFlag.Name)
	mysqlURL := cctx.String(mysqlURLFlag.Name)
	url := cctx.String(dropletURLFlag.Name)
	token := cctx.String(dropletTokenFlag.Name)
	minerAddrStr := cctx.String(minersAddrFlag.Name)
	fmt.Println("mongo url:", mongoURL)
	fmt.Println("mysql url:", mysqlURL)
	fmt.Println("droplet url:", url, "token:", token)
	fmt.Println("miner addr:", minerAddrStr)

	minerAddrs := make(map[address.Address]struct{})
	for _, addr := range strings.Split(minerAddrStr, ",") {
		if len(addr) == 0 {
			continue
		}
		addr, err := address.NewFromString(addr)
		if err != nil {
			return nil, err
		}
		minerAddrs[addr] = struct{}{}
	}

	filter := func(addr address.Address) bool {
		if len(minerAddrs) == 0 {
			return false
		}

		_, ok := minerAddrs[addr]

		return !ok
	}

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
	sort.Slice(deals, func(i, j int) bool {
		return deals[i].CreationTime.Time().After(deals[i].CreationTime.Time())
	})

	start, end, err := getStartEndTime(cctx)
	if err != nil {
		return nil, fmt.Errorf("parse time failed: %v", err)
	}
	pieces := make(map[string]struct{}, len(deals))
	pieceInfos := make([]*pieceInfo, 0, len(deals))
	for _, deal := range deals {
		if start != nil && start.After(deal.CreationTime.Time()) {
			continue
		}
		if end != nil && end.Before(deal.CreationTime.Time()) {
			continue
		}
		if filter(deal.Proposal.Provider) {
			continue
		}
		p := deal.Proposal.PieceCID.String()
		if _, ok := pieces[p]; !ok {
			pieces[p] = struct{}{}
			pieceInfos = append(pieceInfos, &pieceInfo{piece: p, payloadSize: deal.PayloadSize, pieceSize: uint64(deal.Proposal.PieceSize)})
		}
	}

	activeDirectDeal := market.DealActive
	directDeals, err := api.ListDirectDeals(ctx, market.DirectDealQueryParams{State: &activeDirectDeal})
	if err != nil {
		return nil, fmt.Errorf("list direct deal failed: %v", err)
	}
	for _, deal := range directDeals {
		if start != nil && start.After(time.Unix(int64(deal.CreatedAt), 0)) {
			continue
		}
		if end != nil && end.Before(time.Unix(int64(deal.CreatedAt), 0)) {
			continue
		}
		if filter(deal.Provider) {
			continue
		}
		p := deal.PieceCID.String()
		if _, ok := pieces[p]; !ok {
			pieces[p] = struct{}{}
			pieceInfos = append(pieceInfos, &pieceInfo{piece: p, payloadSize: deal.PayloadSize, pieceSize: uint64(deal.PieceSize)})
		}
	}

	fmt.Printf("active deals: %d, valid deals: %d\n", len(deals)+len(directDeals), len(pieceInfos))

	if len(mongoURL) != 0 && topIndexRepo == nil {
		topIndexRepo, err = dagstore.NewMongoTopIndex(ctx, mongoURL)
		if err != nil {
			return nil, fmt.Errorf("connect to mongo failed: %v", err)
		}
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
		pieceInfos:   pieceInfos,
	}, nil
}

func getStartEndTime(cctx *cli.Context) (*time.Time, *time.Time, error) {
	var start, end *time.Time
	if cctx.IsSet(startFlag.Name) {
		t, err := time.Parse("2006-01-02", cctx.String(startFlag.Name))
		if err != nil {
			return nil, nil, err
		}
		start = &t
	}
	if cctx.IsSet(endFlag.Name) {
		t, err := time.Parse("2006-01-02", cctx.String(endFlag.Name))
		if err != nil {
			return nil, nil, err
		}
		end = &t
	}
	fmt.Println("start:", start, "end:", end)
	return start, end, nil
}

func generateIndex(ctx context.Context, carDir string, indexDir string, p *params) error {
	doGenIndex := func(pi *pieceInfo) error {
		piece := pi.piece
		f, err := openCar(carDir, piece)
		if err != nil {
			return err
		}
		defer f.Close() //nolint

		// piece may padding
		r := utils.NewAlgnZeroMountReader(f, int(pi.payloadSize), int(pi.pieceSize))
		idx, err := car.ReadOrGenerateIndex(r, car.ZeroLengthSectionAsEOF(true), car.StoreIdentityCIDs(true))
		if err == nil {
			fmt.Printf("generate index success: %s\n", piece)

			if err := saveIndex(idx, indexDir, piece); err != nil {
				return fmt.Errorf("save index failed, piece: %s, error: %v", piece, err)
			}
			if p.topIndexRepo != nil {
				if err := saveTopIndexToMongo(ctx, piece, idx, p.topIndexRepo); err != nil {
					return fmt.Errorf("save top index to mongo failed, piece: %s, error: %v", piece, err)
				}
			}
			if err := saveShardToMysql(ctx, piece, p.shardRepo); err != nil {
				return fmt.Errorf("save shard to mysql failed, piece: %s, error: %vs", piece, err)
			}
		} else {
			fmt.Printf("generate index failed, piece: %s, error: %v\n", piece, err)
		}
		return nil
	}

	wg := sync.WaitGroup{}
	th := throttle.Fixed(p.concurrency)
	var globalErr error
	for _, pi := range p.pieceInfos {
		pi := pi
		has, err := hasIndex(pi.piece, indexDir)
		if err != nil {
			return err
		}
		if has {
			// fmt.Println("already had index:", pi.piece)
			continue
		}
		if globalErr != nil {
			break
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			err = th.Do(ctx, func(ctx context.Context) error {
				err := doGenIndex(pi)
				if err != nil && os.IsNotExist(err) {
					fmt.Println(err)
					return nil
				}
				return err
			})
			if err != nil {
				globalErr = err
			}
		}()
	}
	wg.Wait()

	return globalErr
}

func openCar(carDir, pieceCID string) (*os.File, error) {
	carPath := filepath.Join(carDir, pieceCID+".car")
	f, err := os.Open(carPath)
	if err == nil {
		return f, nil
	}

	return os.Open(filepath.Join(carDir, pieceCID))
}

func hasIndex(piece string, indexDir string) (bool, error) {
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

	return shardRepo.SaveShard(ctx, &shard)
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
		p, err := paramsFromContext(cctx, nil)
		if err != nil {
			return err
		}

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
		// has, err := p.shardRepo.HasShard(ctx, piece)
		// if err != nil {
		// 	return err
		// }
		// if has {
		// 	// fmt.Println("already had shard:", piece)
		// 	return nil
		// }

		f, err := os.Open(indexPath)
		if err != nil {
			return err
		}
		defer f.Close() //nolint
		idx, err := carindex.ReadFrom(f)
		if err != nil {
			return err
		}

		if p.topIndexRepo != nil {
			if err := saveTopIndexToMongo(ctx, piece, idx, p.topIndexRepo); err != nil {
				return fmt.Errorf("save top index to mongo failed, piece: %s, error: %v", piece, err)
			}
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

var indexInfoCmd = &cli.Command{
	Name:      "index-info",
	Usage:     "show index detail info",
	ArgsUsage: "<index file>",
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() < 1 {
			return fmt.Errorf("must pass index file")
		}

		indexFile := cctx.Args().First()
		f, err := os.Open(indexFile)
		if err != nil {
			return err
		}
		defer f.Close() //nolint

		idx, err := carindex.ReadFrom(f)
		if err != nil {
			return err
		}
		iterableIdx, ok := idx.(carindex.IterableIndex)
		if ok {
			items := make([]struct {
				mhash  string
				blkCid cid.Cid
				offset uint64
			}, 0)
			if err := iterableIdx.ForEach(func(mh multihash.Multihash, offset uint64) error {
				items = append(items, struct {
					mhash  string
					blkCid cid.Cid
					offset uint64
				}{
					mhash:  mh.HexString(),
					blkCid: cid.NewCidV1(cid.Raw, mh),
					offset: offset,
				})
				return nil
			}); err != nil {
				return err
			}
			sort.Slice(items, func(i, j int) bool {
				return items[i].offset < items[j].offset
			})

			for _, item := range items {
				fmt.Printf("block cid: %s, offset: %d\n", item.blkCid, item.offset)
			}
		}

		return nil
	},
}
