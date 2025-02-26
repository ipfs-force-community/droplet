package config

import (
	"time"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-address"
)

const (
	DefaultSimultaneousTransfers = uint64(20)

	HomePath = "~/.droplet"
)

var DefaultMarketConfig = &MarketConfig{
	Home: Home{HomePath},
	Common: Common{
		API: API{
			ListenAddress: "/ip4/127.0.0.1/tcp/41235",
			Timeout:       Duration(30 * time.Second),
		},
		Libp2p: Libp2p{
			ListenAddresses: []string{
				"/ip4/0.0.0.0/tcp/58418",
				"/ip6/::/tcp/0",
			},
			AnnounceAddresses:   []string{},
			NoAnnounceAddresses: []string{},
		},
	},
	// 两种选择: 空或者注释形式
	Mysql: Mysql{
		ConnectionString: "",
		MaxOpenConn:      100,
		MaxIdleConn:      100,
		ConnMaxLifeTime:  "1m",
		Debug:            false,
	},
	PieceStorage: PieceStorage{
		Fs: []*FsPieceStorage{},
	},
	DAGStore: DAGStoreConfig{
		MaxConcurrentIndex:         5,
		MaxConcurrencyStorageCalls: 100,
		GCInterval:                 Duration(0),
	},

	SimultaneousTransfersForRetrieval:        DefaultSimultaneousTransfers,
	SimultaneousTransfersForStoragePerClient: DefaultSimultaneousTransfers,
	SimultaneousTransfersForStorage:          DefaultSimultaneousTransfers,

	CommonProvider: defaultProviderConfig(),
	Miners:         nil,
	Journal:        Journal{Path: "journal"},
	Metrics:        *metrics.DefaultMetricsConfig(),
}

var DefaultMarketClientConfig = &MarketClientConfig{
	Home: Home{"~/.droplet-client"},
	Common: Common{
		API: API{
			ListenAddress: "/ip4/127.0.0.1/tcp/41231/ws",
			Timeout:       Duration(30 * time.Second),
		},
		Libp2p: Libp2p{
			ListenAddresses: []string{
				"/ip4/0.0.0.0/tcp/34123",
				"/ip6/::/tcp/0",
			},
			AnnounceAddresses:   []string{},
			NoAnnounceAddresses: []string{},
		},
	},
	DefaultMarketAddress:              Address(address.Undef),
	SimultaneousTransfersForStorage:   DefaultSimultaneousTransfers,
	SimultaneousTransfersForRetrieval: DefaultSimultaneousTransfers,
}
