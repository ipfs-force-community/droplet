package config

import (
	"github.com/filecoin-project/venus/pkg/types"
	"time"
)

var DefaultMarketConfig = &MarketConfig{
	Home:         Home{"~/.venusmarket"},
	MinerAddress: "f01005",
	Common:       deferCommon,
	Node: Node{
		Url:   "/ip4/192.168.200.12/tcp/3453",
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
	ConsiderOnlineStorageDeals:      false,
	ConsiderOfflineStorageDeals:     false,
	ConsiderOnlineRetrievalDeals:    false,
	ConsiderOfflineRetrievalDeals:   false,
	ConsiderVerifiedStorageDeals:    false,
	ConsiderUnverifiedStorageDeals:  false,
	PieceCidBlocklist:               nil,
	ExpectedSealDuration:            0,
	MaxDealStartDelay:               0,
	PublishMsgPeriod:                0,
	MaxDealsPerPublishMsg:           0,
	MaxProviderCollateralMultiplier: 0,
	SimultaneousTransfers:           0,
	Filter:                          "",
	RetrievalFilter:                 "",
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

var deferCommon = Common{
	API: API{
		ListenAddress: "/ip4/127.0.0.1/tcp/1234/http",
		Timeout:       Duration(30 * time.Second),
	},
	Libp2p: Libp2p{
		ListenAddresses: []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
		},
		AnnounceAddresses:   []string{},
		NoAnnounceAddresses: []string{},
	},
}

var DefaultMarketClientConfig = &MarketClientConfig{
	Home: Home{"~/.venusclient"},
	Libp2p: Libp2p{
		ListenAddresses: []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
		},
		AnnounceAddresses:   []string{},
		NoAnnounceAddresses: []string{},
	},
	Node: Node{
		Url:   "/ip4/192.168.200.12/tcp/3453",
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
}
