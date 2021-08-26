package config

import (
	"github.com/filecoin-project/venus/pkg/types"
	"time"
)

var DefaultMarketConfig = &MarketConfig{
	HomeDir: "~/.venusmarket",
	Common:  deferCommon,
	Node: Node{
		Url:   "/ip4/8.130.165.167/tcp/34530",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic3Rlc3QiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.FEPMm5aKcm7pyn7iDMRl4CEs0-X3MQpgjORPRy9WPso",
	},
	Messager: Messager{
		Url:   "/ip4/8.130.164.80/tcp/39813",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic3Rlc3QiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.FEPMm5aKcm7pyn7iDMRl4CEs0-X3MQpgjORPRy9WPso",
	},
	Gateway: Gateway{
		Url:   "/ip4/8.130.165.167/tcp/45130",
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic3Rlc3QiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.FEPMm5aKcm7pyn7iDMRl4CEs0-X3MQpgjORPRy9WPso",
	},
	DAGStore: DAGStoreConfig{
		MaxConcurrentIndex:         5,
		MaxConcurrencyStorageCalls: 100,
		GCInterval:                 Duration(1 * time.Minute),
	},
	Journal: Journal{Path: "journal"},

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
	RetrievalPricing:                nil,
	MaxPublishDealsFee:              types.FIL{},
	MaxMarketBalanceAddFee:          types.FIL{},
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
