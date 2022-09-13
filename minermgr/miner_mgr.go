package minermgr

import (
	"context"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus-auth/jwtclient"

	"github.com/filecoin-project/venus-market/v2/config"

	"github.com/filecoin-project/venus/venus-shared/types/market"
)

const CoMinersLimit = 500

var log = logging.Logger("user-manager")

type MinerMgrImpl struct {
	authClient *jwtclient.AuthClient
	authNode   config.AuthNode

	miners map[address.Address]*market.User
	lk     sync.Mutex

	localMiners map[address.Address]*market.User
}

var _ IMinerMgr = (*MinerMgrImpl)(nil)

func NewMinerMgrImpl(ctx metrics.MetricsCtx, authClient *jwtclient.AuthClient, cfg *config.MarketConfig) (IMinerMgr, error) {
	m := &MinerMgrImpl{
		authClient: authClient,
		authNode:   cfg.AuthNode,

		miners:      make(map[address.Address]*market.User),
		localMiners: make(map[address.Address]*market.User),
	}

	// storage miner
	for _, miner := range cfg.StorageMiners {
		m.localMiners[address.Address(miner.Addr)] = &market.User{Addr: address.Address(miner.Addr), Account: miner.Account}
	}

	if authClient != nil {
		go m.refreshUsers(ctx)
	}

	return m, nil
}

func (m *MinerMgrImpl) MinerList(context.Context) ([]address.Address, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	miners := make([]address.Address, len(m.miners))
	for miner := range m.miners {
		miners = append(miners, miner)
	}

	for miner := range m.localMiners {
		miners = append(miners, miner)
	}

	return miners, nil
}

func (m *MinerMgrImpl) ActorList(ctx context.Context) ([]market.User, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	users := make([]market.User, 0)
	for _, user := range m.miners {
		users = append(users, market.User{Addr: user.Addr, Account: user.Account})
	}

	for _, user := range m.localMiners {
		users = append(users, market.User{Addr: user.Addr, Account: user.Account})
	}

	return users, nil
}

func (m *MinerMgrImpl) Has(ctx context.Context, mAddr address.Address) bool {
	m.lk.Lock()
	defer m.lk.Unlock()

	_, ok := m.miners[mAddr]
	if ok {
		return ok
	}

	_, ok = m.localMiners[mAddr]
	return ok
}

func (m *MinerMgrImpl) getMinerFromVenusAuth(ctx context.Context, skip, limit int64) error {
	if m.authClient == nil {
		return nil
	}

	if limit == 0 {
		limit = CoMinersLimit
	}

	usersWithMiners, err := m.authClient.ListUsersWithMiners(&auth.ListUsersRequest{
		Page: &core.Page{Skip: skip, Limit: limit},
	})
	if err != nil {
		return err
	}

	// To avoid some deleted or disabled ones that cannot be removed, re-add them here
	m.miners = make(map[address.Address]*market.User)

	for _, u := range usersWithMiners {
		if u.State != core.UserStateEnabled {
			log.Warnf("%s state is: %s, won't list its miners", u.Name, u.State.String())
			continue
		}

		for _, miner := range u.Miners {
			addr, err := address.NewFromString(miner.Miner)
			if err != nil {
				log.Warnf("invalid miner: %s in user: %s", miner.Miner, u.Name)
				continue
			}

			if _, ok := m.miners[addr]; !ok {
				m.miners[addr] = &market.User{Addr: addr, Account: u.Name}
			}
		}
	}

	return nil
}

func (m *MinerMgrImpl) refreshUsers(ctx context.Context) {
	if err := m.getMinerFromVenusAuth(ctx, 0, 0); err != nil {
		log.Errorf("first sync users from venus-auth(%s) failed: %s", m.authNode.Url, err)
	}

	tm := time.NewTicker(time.Minute)
	defer tm.Stop()

	for range tm.C {
		log.Infof("sync users from venus-auth, url: %s\n", m.authNode.Url)

		if err := m.getMinerFromVenusAuth(ctx, 0, 0); err != nil {
			log.Errorf("users from venus-auth(%s) failed:%s", m.authNode.Url, err.Error())
		}
	}
}
