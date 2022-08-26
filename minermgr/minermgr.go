package minermgr

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus-auth/jwtclient"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/metrics"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
)

const CoMinersLimit = 200

var log = logging.Logger("address-manager")

type IAddrMgr interface {
	ActorAddress(ctx context.Context) ([]address.Address, error)
	ActorList(ctx context.Context) ([]types.User, error)
	Has(ctx context.Context, addr address.Address) bool
	GetMiners(ctx context.Context) ([]types.User, error)
	GetAccount(ctx context.Context, addr address.Address) (string, error)
}

type UserMgrImpl struct {
	fullNode v1api.FullNode

	authClient *jwtclient.AuthClient
	authNode   config.AuthNode

	miners []types.User
	lk     sync.Mutex
}

var _ IAddrMgr = (*UserMgrImpl)(nil)

func NeAddrMgrImpl(ctx metrics.MetricsCtx, fullNode v1api.FullNode, authClient *jwtclient.AuthClient, cfg *config.MarketConfig) (IAddrMgr, error) {
	m := &UserMgrImpl{fullNode: fullNode, authClient: authClient, authNode: cfg.AuthNode}

	err := m.addUser(ctx, convertConfigAddress(cfg.StorageMiners)...)
	if err != nil {
		return nil, err
	}

	err = m.addUser(ctx, types.User{
		Addr:    address.Address(cfg.RetrievalPaymentAddress.Addr),
		Account: cfg.RetrievalPaymentAddress.Account,
	})
	if err != nil {
		return nil, err
	}

	err = m.addUser(ctx, convertConfigAddress(cfg.AddressConfig.DealPublishControl)...)
	if err != nil {
		return nil, err
	}

	if m.authClient != nil {
		if err := m.refreshOnce(ctx); err != nil {
			return nil, fmt.Errorf("first refresh users from venus-auth(%s) failed: %w", m.authNode.Url, err)
		}
		go m.refreshUsers(ctx)
	}
	return m, nil
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

func (m *UserMgrImpl) ActorList(ctx context.Context) ([]types.User, error) {
	m.lk.Lock()
	defer m.lk.Unlock()
	users := make([]types.User, len(m.miners))
	for index, user := range m.miners {
		users[index] = types.User{
			Addr:    user.Addr,
			Account: user.Account,
		}
	}
	return users, nil
}

func (m *UserMgrImpl) GetMiners(ctx context.Context) ([]types.User, error) {
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
		return "", errors.Errorf("unable find account of address %s", addr)
	}

	return account, nil
}

func (m *UserMgrImpl) getMinerFromVenusAuth(ctx context.Context, skip, limit int64) ([]types.User, error) {
	if m.authClient == nil {
		return nil, nil
	}
	if limit == 0 {
		limit = CoMinersLimit
	}

	usersWithMiners, err := m.authClient.ListUsersWithMiners(&auth.ListUsersRequest{
		Page: &core.Page{Skip: skip, Limit: limit},
	})

	if err != nil {
		return nil, err
	}

	var users []types.User
	for _, u := range usersWithMiners {
		if u.State != core.UserStateEnabled {
			log.Warnf("user:%s state is: %s, won't list its mienrs", u.Name, u.State.String())
			continue
		}
		for _, m := range u.Miners {
			addr, err := address.NewFromString(m.Miner)
			if err != nil {
				log.Warnf("invalid miner:%s in user:%s", m.Miner, u.Name)
				continue
			}
			users = append(users, types.User{Addr: addr, Account: u.Name})
		}
	}

	return users, nil
}

func (m *UserMgrImpl) addUser(ctx context.Context, usrs ...types.User) error {
	for _, usr := range usrs {
		if usr.Addr == address.Undef {
			continue
		}

		actor, err := m.fullNode.StateGetActor(ctx, usr.Addr, vTypes.EmptyTSK)
		if err != nil {
			return err
		}

		if err = m.appendAddress(ctx, usr.Account, usr.Addr); err != nil {
			return err
		}
		if builtin.IsStorageMinerActor(actor.Code) {
			// add owner/worker/controladdress for this miner
			minerInfo, err := m.fullNode.StateMinerInfo(ctx, usr.Addr, vTypes.EmptyTSK)
			if err != nil {
				return err
			}

			//Notice `multisig` address is not sign-able. we should ignore the `owner`, if it is a `multisig`
			if err = m.appendAddress(ctx, usr.Account, minerInfo.Owner); err != nil {
				return err
			}

			if err = m.appendAddress(ctx, usr.Account, minerInfo.Worker); err != nil {
				return err
			}

			for _, ctlAddr := range minerInfo.ControlAddresses {
				if err = m.appendAddress(ctx, usr.Account, ctlAddr); err != nil {
					return err
				}
			}
		}

	}
	return nil
}

func (m *UserMgrImpl) appendAddress(ctx context.Context, account string, addr address.Address) error {
	m.lk.Lock()
	defer m.lk.Unlock()
	filter := make(map[address.Address]struct{})
	for _, miner := range m.miners {
		filter[miner.Addr] = struct{}{}
	}
	// since `multisig` address is not sign-able.
	//   we should ignore the `owner`, if it is a `multisig`
	actor, err := m.fullNode.StateGetActor(ctx, addr, vTypes.EmptyTSK)
	if err != nil {
		return err
	}

	if builtin.IsAccountActor(actor.Code) {
		accountKey, err := m.fullNode.StateAccountKey(ctx, addr, vTypes.EmptyTSK)
		if err != nil {
			return err
		}
		if _, ok := filter[accountKey]; !ok {
			filter[accountKey] = struct{}{}
			m.miners = append(m.miners, types.User{
				Addr:    accountKey,
				Account: account,
			})
		}
	} else if builtin.IsStorageMinerActor(actor.Code) {
		if _, ok := filter[addr]; !ok {
			filter[addr] = struct{}{}
			m.miners = append(m.miners, types.User{
				Addr:    addr,
				Account: account,
			})
		}
	}
	return nil
}

// todo: looks like refreshUsers only add miners,
//
//	considering venus-auth may delete/disable user(very few, but occurs),
//	the correct way is syncing 'miners' with venus-auth.
func (m *UserMgrImpl) refreshOnce(ctx context.Context) error {
	if m.authClient == nil {
		return errors.Errorf("authClient is nil")
	}

	log.Infof("refresh miners from venus-auth, url: %s\n", m.authNode.Url)
	miners, err := m.getMinerFromVenusAuth(ctx, 0, 0)
	if err != nil {
		return err
	}
	return m.addUser(ctx, miners...)
}

func (m *UserMgrImpl) refreshUsers(ctx context.Context) {
	if m.authClient == nil {
		log.Warnf("auth client is nil, won't refresh users from venus-auth")
		return
	}
	tm := time.NewTicker(time.Minute)
	defer tm.Stop()

	for range tm.C {
		if err := m.refreshOnce(ctx); err != nil {
			log.Errorf("refresh user from auth(%s) failed:%s", m.authNode.Url, err.Error())
		}
	}
}

func convertConfigAddress(addrs []config.User) []types.User {
	addrs2 := make([]types.User, len(addrs))
	for index, miner := range addrs {
		addrs2[index] = types.User{
			Addr:    address.Address(miner.Addr),
			Account: miner.Account,
		}
	}
	return addrs2
}
