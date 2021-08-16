package main

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/storage"
	"github.com/filecoin-project/venus-market/config"
)

func MigrateAddressSelector(cfg *config.Market) (*storage.AddressSelector, error) {
	addrs := make([]address.Address, len(cfg.AddressConfig.DealPublishControl))

	for index, addrStr := range cfg.AddressConfig.DealPublishControl {
		addr, err := address.NewFromString(addrStr)
		if err != nil {
			return nil, err
		}
		addrs[index] = addr
	}
	return &storage.AddressSelector{
		AddressConfig: api.AddressConfig{
			PreCommitControl:      nil,
			CommitControl:         nil,
			TerminateControl:      nil,
			DealPublishControl:    addrs,
			DisableOwnerFallback:  cfg.AddressConfig.DisableOwnerFallback,
			DisableWorkerFallback: cfg.AddressConfig.DisableWorkerFallback,
		},
	}, nil
}
