package minermgr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/go-resty/resty/v2"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus-market/config"
)

const CoMinersLimit = 200

var log = logging.Logger("miner-manager")

type IMinerMgr interface {
	ActorAddress(ctx context.Context) ([]address.Address, error)
	Has(ctx context.Context, addr address.Address) bool
	GetMinerFromVenusAuth(ctx context.Context, skip, limit int64) ([]address.Address, error)
}

type MinerMgrImpl struct {
	authCfg config.AuthNode

	miners []address.Address
	lk     sync.Mutex
}

func NewMinerMgrImpl(cfg *config.MarketConfig) func() (IMinerMgr, error) {
	return func() (IMinerMgr, error) {
		m := &MinerMgrImpl{authCfg: cfg.AuthNode}
		err := m.distAddress(config.ConvertConfigAddress(cfg.MinerAddress)...)
		if err != nil {
			return nil, err
		}
		miners, err := m.GetMinerFromVenusAuth(context.TODO(), 0, 0)
		if err != nil {
			return nil, err
		}
		return m, m.distAddress(miners...)
	}
}

func (m *MinerMgrImpl) ActorAddress(ctx context.Context) ([]address.Address, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	return m.miners, nil
}

func (m *MinerMgrImpl) Has(ctx context.Context, addr address.Address) bool {
	m.lk.Lock()
	defer m.lk.Unlock()

	for _, miner := range m.miners {
		if miner.String() == addr.String() {
			return true
		}
	}

	return false
}

func (m *MinerMgrImpl) GetMinerFromVenusAuth(ctx context.Context, skip, limit int64) ([]address.Address, error) {
	log.Infof("request miners from auth: %v ...", m.authCfg)
	if len(m.authCfg.Url) == 0 {
		return []address.Address{}, nil
	}
	if limit == 0 {
		limit = CoMinersLimit
	}
	cli := resty.New().SetHostURL(m.authCfg.Url).SetHeader("Accept", "application/json")
	response, err := cli.R().SetQueryParams(map[string]string{
		"token": m.authCfg.Token,
		"skip":  fmt.Sprintf("%d", skip),
		"limit": fmt.Sprintf("%d", limit),
	}).Get("/user/list")
	if err != nil {
		return nil, err
	}

	switch response.StatusCode() {
	case http.StatusOK:
		var res []User
		err = json.Unmarshal(response.Body(), &res)
		if err != nil {
			return nil, err
		}

		m.lk.Lock()
		m.miners = make([]address.Address, 0)
		for _, val := range res {
			if strings.Index(val.Miner, "f") == 0 || strings.Index(val.Miner, "t") == 0 {
				addr, err := address.NewFromString(val.Miner)
				if err == nil {
					m.miners = append(m.miners, addr)
				} else {
					log.Errorf("miner [%s] is error", val.Miner)
				}
			}
		}
		m.lk.Unlock()
		return m.miners, err
	default:
		response.Result()
		return nil, fmt.Errorf("response code is : %d, msg:%s", response.StatusCode(), response.Body())
	}
}

func (m *MinerMgrImpl) distAddress(addrs ...address.Address) error {
	m.lk.Lock()
	defer m.lk.Unlock()
	filter := make(map[address.Address]struct{}, len(m.miners))
	for _, mAddr := range m.miners {
		filter[mAddr] = struct{}{}
	}

	for _, addr := range addrs {
		if _, ok := filter[addr]; !ok {
			filter[addr] = struct{}{}
			m.miners = append(m.miners, addr)
		}
	}
	return nil
}
