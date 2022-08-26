package config

import (
	"time"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
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
	Node: Node{
		Url:   "", // "/ip4/<ip>/tcp/3453",
		Token: "",
	},
	Messager: Messager{
		Url:   "", // /ip4/<ip>/tcp/39812
		Token: "",
	},
	Signer: Signer{
		Url:   "", // /ip4/<ip>/tcp/5678
		Token: "",
	},
	Mysql: Mysql{
		ConnectionString: "",
		MaxOpenConn:      100,
		MaxIdleConn:      100,
		ConnMaxLifeTime:  "1m",
		Debug:            false,
	},
	AuthNode: AuthNode{
		Url:   "", // "http://<ip>:8989",
		Token: "",
	},
	DAGStore: DAGStoreConfig{
		MaxConcurrentIndex:         5,
		MaxConcurrencyStorageCalls: 100,
		GCInterval:                 Duration(1 * time.Minute),
	},
	Journal: Journal{Path: "journal"},
	PieceStorage: PieceStorage{
		Fs: []*FsPieceStorage{},
	},
	ConsiderOnlineStorageDeals:     true,
	ConsiderOfflineStorageDeals:    true,
	ConsiderOnlineRetrievalDeals:   true,
	ConsiderOfflineRetrievalDeals:  true,
	ConsiderVerifiedStorageDeals:   true,
	ConsiderUnverifiedStorageDeals: true,
	PieceCidBlocklist:              []cid.Cid{},
	// TODO: It'd be nice to set this based on sector size
	MaxDealStartDelay:    Duration(time.Hour * 24 * 14),
	ExpectedSealDuration: Duration(time.Hour * 24),
	PublishMsgPeriod:     Duration(time.Hour),

	MaxDealsPerPublishMsg:           8,
	MaxProviderCollateralMultiplier: 2,

	SimultaneousTransfersForRetrieval:        DefaultSimultaneousTransfers,
	SimultaneousTransfersForStoragePerClient: DefaultSimultaneousTransfers,
	SimultaneousTransfersForStorage:          DefaultSimultaneousTransfers,

	RetrievalPricing: &RetrievalPricing{
		Strategy: RetrievalPricingDefaultMode,
		Default: &RetrievalPricingDefault{
			VerifiedDealsFreeTransfer: true,
		},
		External: &RetrievalPricingExternal{
			Path: "",
		},
	},

	MaxPublishDealsFee:     types.FIL(types.NewInt(0)),
	MaxMarketBalanceAddFee: types.FIL(types.NewInt(0)),
	Metrics:                *metrics.DefaultMetricsConfig(),
}

var DefaultMarketClientConfig = &MarketClientConfig{
	Home: Home{"~/.venusclient"},
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
		Url:   "", // "/ip4/<ip>/tcp/3453",
		Token: "",
	},
	Signer: Signer{
		Url:   "", // "/ip4/<ip>/tcp/5678",
		Token: "",
	},
	Messager: Messager{
		Url:   "", // "/ip4/<ip>/tcp/39812",
		Token: "",
	},
	DefaultMarketAddress:              Address(address.Undef),
	SimultaneousTransfersForStorage:   DefaultSimultaneousTransfers,
	SimultaneousTransfersForRetrieval: DefaultSimultaneousTransfers,
}
