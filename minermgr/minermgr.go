package minermgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/xerrors"
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
	GetAccount(ctx context.Context, addr address.Address) (string, error)
	GetMinerFromVenusAuth(ctx context.Context, skip, limit int64) ([]Miner, error)
}

type Miner struct {
	Addr    address.Address
	Account string
}

type MinerMgrImpl struct {
	authCfg config.AuthNode

	miners []Miner
	lk     sync.Mutex
}

func NewMinerMgrImpl(cfg *config.MarketConfig) func() (IMinerMgr, error) {
	return func() (IMinerMgr, error) {
		m := &MinerMgrImpl{authCfg: cfg.AuthNode}
		err := m.distAddress(convertConfigAddress(cfg.StorageMiners)...)
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
	addrs := make([]address.Address, len(m.miners))
	for index, user := range m.miners {
		addrs[index] = user.Addr
	}
	return addrs, nil
}

func (m *MinerMgrImpl) Has(ctx context.Context, addr address.Address) bool {
	m.lk.Lock()
	defer m.lk.Unlock()

	for _, miner := range m.miners {
		if bytes.Equal(miner.Addr.Bytes(), addr.Bytes()) {
			return true
		}
	}

	return false
}

func (m *MinerMgrImpl) GetAccount(ctx context.Context, addr address.Address) (string, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	var account string
	for _, miner := range m.miners {
		if bytes.Equal(miner.Addr.Bytes(), addr.Bytes()) {
			account = miner.Account
		}
	}

	if len(account) == 0 {
		return "", xerrors.Errorf("find account of address %s", addr)
	}

	return account, nil
}

func (m *MinerMgrImpl) GetMinerFromVenusAuth(ctx context.Context, skip, limit int64) ([]Miner, error) {
	log.Infof("request miners from auth: %v ...", m.authCfg)
	if len(m.authCfg.Url) == 0 {
		return nil, nil
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
		m.miners = make([]Miner, 0)
		for _, val := range res {
			if len(val.Miner) > 0 {
				addr, err := address.NewFromString(val.Miner)
				if err == nil && addr != address.Undef {
					m.miners = append(m.miners, Miner{
						Addr:    addr,
						Account: val.Miner,
					})
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

func (m *MinerMgrImpl) distAddress(addrs ...Miner) error {
	m.lk.Lock()
	defer m.lk.Unlock()
	filter := make(map[address.Address]struct{}, len(m.miners))
	for _, miner := range m.miners {
		filter[miner.Addr] = struct{}{}
	}

	for _, miner := range addrs {
		if _, ok := filter[miner.Addr]; !ok {
			filter[miner.Addr] = struct{}{}
			m.miners = append(m.miners, miner)
		}
	}
	return nil
}

func convertConfigAddress(addrs []config.Miner) []Miner {
	addrs2 := make([]Miner, len(addrs))
	for index, miner := range addrs {
		addrs2[index] = Miner{
			Addr:    address.Address(miner.Addr),
			Account: miner.Account,
		}
	}
	return addrs2
}
