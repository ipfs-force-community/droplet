package config

import (
	"time"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

// API contains configs for API endpoint
type API struct {
	ListenAddress       string
	RemoteListenAddress string
	Timeout             Duration

	PrivateKey string
}

// Libp2p contains configs for libp2p
type Libp2p struct {
	// Binding address for the libp2p host - 0 means random port.
	// Format: multiaddress; see https://multiformats.io/multiaddr/
	ListenAddresses []string
	// Addresses to explicitally announce to other peers. If not specified,
	// all interface addresses are announced
	// Format: multiaddress
	AnnounceAddresses []string
	// Addresses to not announce
	// Format: multiaddress
	NoAnnounceAddresses []string
	ProtectedPeers      []string

	PrivateKey string
}

type Common struct {
	API    API
	Libp2p Libp2p
}

// ConnectConfig chain-service connect config
type ConnectConfig struct {
	Url   string
	Token string
}

type (
	Node     ConnectConfig
	Messager ConnectConfig
	AuthNode ConnectConfig
)

type SignerType = string

const (
	SignerTypeLotusnode = "lotusnode"
	SignerTypeWallet    = "wallet"
	SignerTypeGateway   = "gateway"
)

type Signer struct {
	SignerType SignerType `toml:"Type"`
	Url        string
	Token      string
}

// ProviderConfig is common config for provider
type ProviderConfig struct {
	// When enabled, the miner can accept online deals
	ConsiderOnlineStorageDeals bool
	// When enabled, the miner can accept offline deals
	ConsiderOfflineStorageDeals bool
	// When enabled, the miner can accept retrieval deals
	ConsiderOnlineRetrievalDeals bool
	// When enabled, the miner can accept offline retrieval deals
	ConsiderOfflineRetrievalDeals bool
	// When enabled, the miner can accept verified deals
	ConsiderVerifiedStorageDeals bool
	// When enabled, the miner can accept unverified deals
	ConsiderUnverifiedStorageDeals bool
	// A list of Data CIDs to reject when making deals
	PieceCidBlocklist []cid.Cid
	// Maximum expected amount of time getting the deal into a sealed sector will take
	// This includes the time the deal will need to get transferred and published
	// before being assigned to a sector
	ExpectedSealDuration Duration
	// Maximum amount of time proposed deal StartEpoch can be in future
	MaxDealStartDelay Duration
	// todo 以上参数在目前实现中没有起到实际的作用???

	// todo 以下参数缺少配置API???
	// When a deal is ready to publish, the amount of time to wait for more
	// deals to be ready to publish before publishing them all as a batch
	PublishMsgPeriod Duration
	// The maximum number of deals to include in a single PublishStorageDeals
	// message
	MaxDealsPerPublishMsg uint64

	// The maximum collateral that the provider will put up against a deal,
	// as a multiplier of the minimum collateral bound
	MaxProviderCollateralMultiplier uint64

	// A command used for fine-grained evaluation of piecestorage deals
	// see https://docs.filecoin.io/mine/lotus/miner-configuration/#using-filters-for-fine-grained-storage-and-retrieval-deal-acceptance for more details
	Filter string
	// A command used for fine-grained evaluation of retrieval deals
	// see https://docs.filecoin.io/mine/lotus/miner-configuration/#using-filters-for-fine-grained-storage-and-retrieval-deal-acceptance for more details
	RetrievalFilter string

	TransferPath string

	RetrievalPricing *RetrievalPricing // todo reserve

	MaxPublishDealsFee     types.FIL
	MaxMarketBalanceAddFee types.FIL

	RetrievalPaymentAddress Address

	DealPublishAddress []Address

	// The public multi-address for retrieving deals with droplet-http.
	// Note: Must be in multiaddr format, eg /ip4/127.0.0.1/tcp/53241/http
	HTTPRetrievalMultiaddr string
}

func defaultProviderConfig() *ProviderConfig {
	return &ProviderConfig{
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

		Filter:          "",
		RetrievalFilter: "",

		TransferPath: "",

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
	}
}
