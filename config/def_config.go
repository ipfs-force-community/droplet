package config

import (
	"time"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-address"
)

const (
	DefaultSimultaneousTransfers = uint64(20)

	HomePath = "~/.venusmarket"
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
	Node: Node{
		Url:   "", // /ip4/<ip>/tcp/3453
		Token: "", // cs-token
	},
	Messager: Messager{
		Url:   "", // /ip4/<ip>/tcp/39812
		Token: "", // cs-token
	},
	AuthNode: AuthNode{
		Url:   "", // http://<ip>:8989
		Token: "", // cs-token
	},
	Signer: Signer{
		SignerType: "wallet",
		Url:        "", // /ip4/<ip>/tcp/5678
		Token:      "", // signer-token
	},
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
		GCInterval:                 Duration(1 * time.Minute),
	},

	SimultaneousTransfersForRetrieval:        DefaultSimultaneousTransfers,
	SimultaneousTransfersForStoragePerClient: DefaultSimultaneousTransfers,
	SimultaneousTransfersForStorage:          DefaultSimultaneousTransfers,

	CommonProvider: defaultProviderConfig(),
	Miners:         make([]*MinerConfig, 0),
	Journal:        Journal{Path: "journal"},
	Metrics:        *metrics.DefaultMetricsConfig(),
}

var DefaultMarketClientConfig = &MarketClientConfig{
	Home: Home{"~/.marketclient"},
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
	Node: Node{
		Url:   "", // /ip4/<ip>/tcp/3453
		Token: "", // cs-token
	},
	Signer: Signer{
		SignerType: "wallet",
		Url:        "", // /ip4/<ip>/tcp/5678
		Token:      "", // signer-token
	},
	Messager: Messager{
		Url:   "", // /ip4/<ip>/tcp/39812
		Token: "", // cs-token
	},
	DefaultMarketAddress:              Address(address.Undef),
	SimultaneousTransfersForStorage:   DefaultSimultaneousTransfers,
	SimultaneousTransfersForRetrieval: DefaultSimultaneousTransfers,
}
