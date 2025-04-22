package storageprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/metrics"
	"github.com/ipfs-force-community/droplet/v2/minermgr"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	metrics2 "github.com/ipfs-force-community/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.uber.org/fx"
)

type DealMetric struct {
	r        repo.Repo
	minerMgr minermgr.IMinerMgr
	dagStore *dagstore.DAGStore
}

func NewDealMetric(mCtx metrics2.MetricsCtx,
	lc fx.Lifecycle,
	r repo.Repo,
	minerMgr minermgr.IMinerMgr,
	dagStore *dagstore.DAGStore,
) *DealMetric {
	dm := &DealMetric{
		r:        r,
		minerMgr: minerMgr,
		dagStore: dagStore,
	}

	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		ctx = metrics2.LifecycleCtx(mCtx, lc)
		go func() {
			dm.Start(ctx)
		}()
		return nil
	}})

	return dm
}

func (dm *DealMetric) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Hour * 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			minerAddrs, err := dm.minerMgr.ActorList(ctx)
			if err != nil {
				log.Errorf("get actor list failed: %v", err)
				continue
			}
			for _, minerAddr := range minerAddrs {
				dm.sparkData(ctx, minerAddr.Addr.String())
			}

			if err := dm.dealActiveInfo(ctx, minerAddrs); err != nil {
				log.Errorf("get miner active deal info failed: %v", err)
				continue
			}

			var count int
			shards := dm.dagStore.AllShardsInfo()
			for _, shard := range shards {
				if shard.ShardState == dagstore.ShardStateServing || shard.ShardState == dagstore.ShardStateAvailable {
					count++
				}
			}
			stats.Record(ctx, metrics.DagStoreActiveShardCount.M(int64(count)))
		}
	}
}

func (dm *DealMetric) dealActiveInfo(ctx context.Context, miners []market.User) error {
	for _, miner := range miners {
		minerAddr := miner.Addr
		c, err := dm.r.StorageDealRepo().CountDealByMiner(ctx, minerAddr, storagemarket.StorageDealActive)
		if err != nil {
			return err
		}
		dc, err := dm.r.DirectDealRepo().CountDealByMiner(ctx, minerAddr, market.DealActive)
		if err != nil {
			return err
		}
		_ = stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(metrics.MinerAddressTag, minerAddr.String())},
			metrics.ActiveDealCount.M(int64(dc+c)))
		log.Infof("miner %s active deal count: %d + %d", minerAddr, c, dc)
	}

	return nil
}

func (dm *DealMetric) sparkData(ctx context.Context, minerAddr string) {
	// get miner retrieval rate
	retrievalRate, err := getMinerRetrievalRate(ctx, minerAddr)
	if err != nil {
		log.Warnf("get miner %s retrieval rate failed: %v", minerAddr, err)
	} else {
		_ = stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(metrics.MinerAddressTag, minerAddr)},
			metrics.SparkRetrievalRate.M(int64(retrievalRate*100)))
	}

	// get miner eligible deal count
	dealCount, err := getMinerEligibleDealCount(ctx, minerAddr)
	if err != nil {
		log.Warnf("get miner %s eligible deal count failed: %v", minerAddr, err)
	} else {
		_ = stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(metrics.MinerAddressTag, minerAddr)},
			metrics.SparkEligibleDealCount.M(int64(dealCount)))
	}
}

// https://stats.filspark.com/miner/f02002200/retrieval-success-rate/summary?from=2025-03-20&to=2025-03-30
// [{"day":"2025-03-20","total":"2426","successful":"2039","success_rate":0.8404781533388294,"successful_http":"2039","success_rate_http":0.8404781533388294,"success_rate_http_head":0}]
func getMinerRetrievalRate(ctx context.Context, minerAddr string) (float64, error) {
	cli := http.DefaultClient
	cli.Timeout = time.Second * 60
	day := time.Now().Format(time.DateOnly)

	url := fmt.Sprintf("https://stats.filspark.com/miner/%s/retrieval-success-rate/summary?from=%s&to=%s", minerAddr, day, day)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get miner retrieval rate failed: %v", resp.Status)
	}
	var ret []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return 0, err
	}
	if len(ret) == 0 {
		return 0, fmt.Errorf("empty response")
	}

	log.Infof("get miner retrieval rate: %v", ret)

	first := ret[0]
	count, ok := first["success_rate"]
	if !ok {
		return 0, fmt.Errorf("parse miner retrieval rate failed: %v", first["success_rate"])
	}
	countFloat, ok := count.(float64)
	if !ok {
		return 0, fmt.Errorf("parse miner retrieval rate failed: %v", first["success_rate"])
	}

	return countFloat, nil
}

// https://api.filspark.com/miner/{MinerID}/deals/eligible/summary
// response {"minerId":"f03519255","dealCount":554,"clients":[{"clientId":"f03524784","dealCount":554}]}
func getMinerEligibleDealCount(ctx context.Context, minerAddr string) (int, error) {
	cli := http.DefaultClient
	cli.Timeout = time.Second * 60

	url := fmt.Sprintf("https://api.filspark.com/miner/%s/deals/eligible/summary", minerAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get miner eligible deal count failed: %v", resp.Status)
	}

	var ret map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return 0, err
	}
	log.Infof("get miner eligible deal: %v", ret)

	count, ok := ret["dealCount"]
	if !ok {
		return 0, fmt.Errorf("parse miner eligible deal count failed: %v", ret["dealCount"])
	}
	countInt, ok := count.(float64)
	if !ok {
		return 0, fmt.Errorf("parse miner eligible deal count failed: %v", ret["dealCount"])
	}

	return int(countInt), nil
}
