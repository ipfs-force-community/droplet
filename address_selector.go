package main

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/sealer"
)

func AddressSelector(cfg *config.MarketConfig) (*sealer.AddressSelector, error) {
	return &sealer.AddressSelector{
		AddressConfig: cfg.AddressConfig,
	}, nil
}
