package minermgr

import (
	"context"
	"sync"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-market/v2/config"

	"github.com/filecoin-project/venus/venus-shared/types/market"
)

type MinerMgrImpl struct {
	miners map[address.Address]*market.User
	lk     sync.Mutex
}

var _ IMinerMgr = (*MinerMgrImpl)(nil)

func NewMinerMgrImpl(_ metrics.MetricsCtx, cfg *config.MarketConfig) (IMinerMgr, error) {
	m := &MinerMgrImpl{
		miners: make(map[address.Address]*market.User),
	}

	// storage miner
	for _, miner := range cfg.Miners {
		m.miners[address.Address(miner.Addr)] = &market.User{Addr: address.Address(miner.Addr), Account: miner.Account}
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

	return miners, nil
}

func (m *MinerMgrImpl) ActorList(_ context.Context) ([]market.User, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	users := make([]market.User, 0)
	for _, user := range m.miners {
		users = append(users, market.User{Addr: user.Addr, Account: user.Account})
	}

	return users, nil
}

func (m *MinerMgrImpl) Has(_ context.Context, mAddr address.Address) bool {
	m.lk.Lock()
	defer m.lk.Unlock()

	_, ok := m.miners[mAddr]
	return ok
}
