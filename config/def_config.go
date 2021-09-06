package config

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"time"
)

const (
	DefaultSimultaneousTransfers = uint64(20)
)

var DefaultMarketConfig = &MarketConfig{
	Home:         Home{"~/.venusmarket"},
	MinerAddress: "f01005",
	Common: Common{
		API: API{
			ListenAddress: "/ip4/127.0.0.1/tcp/41235/http",
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
		Url:   "/ip4/192.168.200.14/tcp/3453",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoibGkiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.eJBpUoP6leCSkhWHuy8SliHJUfw5XM7M7BndY3YRVvg",
	},
	Messager: Messager{
		Url:   "/ip4/192.168.200.12/tcp/39812",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoibGkiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.eJBpUoP6leCSkhWHuy8SliHJUfw5XM7M7BndY3YRVvg",
	},
	Signer: Signer{
		Url:   "/ip4/127.0.0.1/tcp/5678/http",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIl19.Y03rbv28jVsXK9t4Ih9a0YmmzGoG2fwa5Ek1VkQByQ0",
	},
	Sealer: Sealer{
		Url:   "/ip4/127.0.0.1/tcp/2345",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.0yOKuJxNwI6wVZybM8jwIvUI_M2oZKAyEpeoTG6qN4M",
	},
	DAGStore: DAGStoreConfig{
		MaxConcurrentIndex:         5,
		MaxConcurrencyStorageCalls: 100,
		GCInterval:                 Duration(1 * time.Minute),
	},
	Journal: Journal{Path: "journal"},
	PieceStorage: PieceStorage{
		Type: "local",
		Path: "/Users/lijunlong/.venusmarket",
	},
	ConsiderOnlineStorageDeals:     true,
	ConsiderOfflineStorageDeals:    true,
	ConsiderOnlineRetrievalDeals:   true,
	ConsiderOfflineRetrievalDeals:  true,
	ConsiderVerifiedStorageDeals:   true,
	ConsiderUnverifiedStorageDeals: true,
	PieceCidBlocklist:              []cid.Cid{},
	// TODO: It'd be nice to set this based on sector size
	MaxDealStartDelay:               Duration(time.Hour * 24 * 14),
	ExpectedSealDuration:            Duration(time.Hour * 24),
	PublishMsgPeriod:                Duration(time.Hour),
	MaxDealsPerPublishMsg:           8,
	MaxProviderCollateralMultiplier: 2,

	SimultaneousTransfers: DefaultSimultaneousTransfers,

	RetrievalPricing: &RetrievalPricing{
		Strategy: RetrievalPricingDefaultMode,
		Default: &RetrievalPricingDefault{
			VerifiedDealsFreeTransfer: true,
		},
		External: &RetrievalPricingExternal{
			Path: "",
		},
	},

	MaxPublishDealsFee:     types.FIL{},
	MaxMarketBalanceAddFee: types.FIL{},
}

var (
	defaultAddr, _ = address.NewFromString("t3wtmlylzuc7ttbhwppjt7m55p7ywybebdx7kfbwx7ijhthqssptvs746njt22i3xe65zw7hyafutmdoobkcoq")
)
var DefaultMarketClientConfig = &MarketClientConfig{
	Home: Home{"~/.venusclient"},
	Common: Common{
		API: API{
			ListenAddress: "/ip4/127.0.0.1/tcp/41231/http",
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
		Url:   "/ip4/192.168.200.14/tcp/3453",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoibGkiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.eJBpUoP6leCSkhWHuy8SliHJUfw5XM7M7BndY3YRVvg",
	},
	Signer: Signer{
		Url:   "/ip4/127.0.0.1/tcp/5678/http",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIl19.Y03rbv28jVsXK9t4Ih9a0YmmzGoG2fwa5Ek1VkQByQ0",
	},
	Market: Market{
		Url:   "/ip4/127.0.0.1/tcp/3453",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoibGkiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.eJBpUoP6leCSkhWHuy8SliHJUfw5XM7M7BndY3YRVvg",
	},
	Messager: Messager{
		Url:   "/ip4/192.168.200.12/tcp/39812",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoibGkiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.eJBpUoP6leCSkhWHuy8SliHJUfw5XM7M7BndY3YRVvg",
	},
	DefaultMarketAddress: defaultAddr,
}
