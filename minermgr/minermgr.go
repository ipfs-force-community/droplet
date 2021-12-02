package minermgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	"golang.org/x/xerrors"
	"net/http"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/go-resty/resty/v2"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus-market/config"
)

const CoMinersLimit = 200

var log = logging.Logger("address-manager")

type IAddrMgr interface {
	ActorAddress(ctx context.Context) ([]address.Address, error)
	Has(ctx context.Context, addr address.Address) bool
	GetMiners(ctx context.Context) ([]User, error)
	GetAccount(ctx context.Context, addr address.Address) (string, error)
	AddAddress(ctx context.Context, user User) error
}

type User struct {
	Addr    address.Address
	Account string
}

type UserMgrImpl struct {
	authCfg  config.AuthNode
	fullNode apiface.FullNode

	miners []User
	lk     sync.Mutex
}

var _ IAddrMgr = (*UserMgrImpl)(nil)

func NeAddrMgrImpl(ctx metrics.MetricsCtx, fullNode apiface.FullNode, cfg *config.MarketConfig) (IAddrMgr, error) {
	m := &UserMgrImpl{authCfg: cfg.AuthNode, fullNode: fullNode}

	err := m.distAddress(ctx, convertConfigAddress(cfg.StorageMiners)...)
	if err != nil {
		return nil, err
	}

	err = m.distAddress(ctx, User{
		Addr:    address.Address(cfg.RetrievalPaymentAddress.Addr),
		Account: cfg.RetrievalPaymentAddress.Account,
	})
	if err != nil {
		return nil, err
	}

	err = m.distAddress(ctx, convertConfigAddress(cfg.AddressConfig.DealPublishControl)...)
	if err != nil {
		return nil, err
	}
	miners, err := m.getMinerFromVenusAuth(context.TODO(), 0, 0)
	if err != nil {
		return nil, err
	}

	return m, m.distAddress(ctx, miners...)

}

func (m *UserMgrImpl) ActorAddress(ctx context.Context) ([]address.Address, error) {
	m.lk.Lock()
	defer m.lk.Unlock()
	addrs := make([]address.Address, len(m.miners))
	for index, user := range m.miners {
		addrs[index] = user.Addr
	}
	return addrs, nil
}

func (m *UserMgrImpl) GetMiners(ctx context.Context) ([]User, error) {
	m.lk.Lock()
	defer m.lk.Unlock()
	return m.miners, nil
}

func (m *UserMgrImpl) Has(ctx context.Context, addr address.Address) bool {
	m.lk.Lock()
	defer m.lk.Unlock()

	for _, miner := range m.miners {
		if bytes.Equal(miner.Addr.Bytes(), addr.Bytes()) {
			return true
		}
	}

	return false
}

func (m *UserMgrImpl) GetAccount(ctx context.Context, addr address.Address) (string, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	var account string
	for _, miner := range m.miners {
		if bytes.Equal(miner.Addr.Bytes(), addr.Bytes()) {
			account = miner.Account
		}
	}

	if len(account) == 0 {
		return "", xerrors.Errorf("unable find account of address %s", addr)
	}

	return account, nil
}

func (m *UserMgrImpl) getMinerFromVenusAuth(ctx context.Context, skip, limit int64) ([]User, error) {
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
		var res []AuthUser
		err = json.Unmarshal(response.Body(), &res)
		if err != nil {
			return nil, err
		}

		m.lk.Lock()
		var miners []User
		for _, val := range res {
			if len(val.Miner) > 0 {
				addr, err := address.NewFromString(val.Miner)
				if err == nil && addr != address.Undef {
					miners = append(miners, User{
						Addr:    addr,
						Account: val.Name,
					})
				} else {
					log.Warnf("miner [%s] is error", val.Miner)
				}
			}
		}
		m.lk.Unlock()
		return miners, err
	default:
		response.Result()
		return nil, fmt.Errorf("response code is : %d, msg:%s", response.StatusCode(), response.Body())
	}
}

func (m *UserMgrImpl) AddAddress(ctx context.Context, user User) error {
	return m.distAddress(ctx, user)
}

func (m *UserMgrImpl) distAddress(ctx context.Context, addrs ...User) error {
	m.lk.Lock()
	defer m.lk.Unlock()
	filter := make(map[address.Address]struct{}, len(m.miners))
	for _, miner := range m.miners {
		filter[miner.Addr] = struct{}{}
	}

	for _, usr := range addrs {
		if usr.Addr == address.Undef {
			continue
		}
		if _, ok := filter[usr.Addr]; !ok {
			filter[usr.Addr] = struct{}{}
			m.miners = append(m.miners, usr)
		}
		actor, err := m.fullNode.StateGetActor(ctx, usr.Addr, types.EmptyTSK)
		if err != nil {
			return err
		}

		if builtin.IsStorageMinerActor(actor.Code) {
			// add owner/worker/controladdress for this miner
			minerInfo, err := m.fullNode.StateMinerInfo(ctx, usr.Addr, types.EmptyTSK)
			if err != nil {
				return err
			}

			workerKey, err := m.fullNode.StateAccountKey(ctx, minerInfo.Worker, types.EmptyTSK)
			if err != nil {
				return err
			}
			if _, ok := filter[workerKey]; !ok {
				filter[workerKey] = struct{}{}
				m.miners = append(m.miners, User{
					Addr:    workerKey,
					Account: usr.Account,
				})
			}

			ownerKey, err := m.fullNode.StateAccountKey(ctx, minerInfo.Owner, types.EmptyTSK)
			if err != nil {
				return err
			}
			if _, ok := filter[ownerKey]; !ok {
				filter[ownerKey] = struct{}{}
				m.miners = append(m.miners, User{
					Addr:    ownerKey,
					Account: usr.Account,
				})
			}

			for _, ctlAddr := range minerInfo.ControlAddresses {
				ctlKey, err := m.fullNode.StateAccountKey(ctx, ctlAddr, types.EmptyTSK)
				if err != nil {
					return err
				}
				if _, ok := filter[ctlKey]; !ok {
					filter[ctlKey] = struct{}{}
					m.miners = append(m.miners, User{
						Addr:    ctlKey,
						Account: usr.Account,
					})
				}
			}
		}

	}
	return nil
}

func convertConfigAddress(addrs []config.User) []User {
	addrs2 := make([]User, len(addrs))
	for index, miner := range addrs {
		addrs2[index] = User{
			Addr:    address.Address(miner.Addr),
			Account: miner.Account,
		}
	}
	return addrs2
}
