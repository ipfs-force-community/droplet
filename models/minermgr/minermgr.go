package minermgr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	UpdateAddress(ctx context.Context, skip, limit int64) ([]address.Address, error)
}

type MinerMgrImpl struct {
	cli   *resty.Client
	token string

	miners []address.Address
	lk     sync.Mutex
}

func NewMinerMgrImpl(cfg *config.MarketConfig) func() (IMinerMgr, error) {
	return func() (IMinerMgr, error) {
		cli := resty.New().SetHostURL(cfg.AuthNode.Url).SetHeader("Accept", "application/json")
		m := &MinerMgrImpl{cli: cli, token: cfg.AuthNode.Token}

		miners, err := m.UpdateAddress(context.TODO(), 0, 0)
		if err != nil {
			return nil, err
		}

		m.miners = miners
		return m, nil
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

func (m *MinerMgrImpl) UpdateAddress(ctx context.Context, skip, limit int64) ([]address.Address, error) {
	if limit == 0 {
		limit = CoMinersLimit
	}

	response, err := m.cli.R().SetQueryParams(map[string]string{
		"token": m.token,
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
			addr, err := address.NewFromString(val.Miner)
			if err == nil {
				m.miners = append(m.miners, addr)
			} else {
				log.Errorf("miner [%s] is error", val.Miner)
			}
		}
		m.lk.Unlock()
		return m.miners, err
	default:
		response.Result()
		return nil, fmt.Errorf("response code is : %d, msg:%s", response.StatusCode(), response.Body())
	}
}
