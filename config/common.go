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
	ChainService ConnectConfig
	Node         ConnectConfig
	Messager     ConnectConfig
	AuthNode     ConnectConfig
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

type IndexProviderConfig struct {
	// Enable set whether to enable indexing announcement to the network and expose endpoints that
	// allow indexer nodes to process announcements. Enabled by default.
	Enable bool

	// EntriesCacheCapacity sets the maximum capacity to use for caching the indexing advertisement
	// entries. Defaults to 1024 if not specified. The cache is evicted using LRU policy. The
	// maximum storage used by the cache is a factor of EntriesCacheCapacity, EntriesChunkSize and
	// the length of multihashes being advertised. For example, advertising 128-bit long multihashes
	// with the default EntriesCacheCapacity, and EntriesChunkSize means the cache size can grow to
	// 256MiB when full.
	EntriesCacheCapacity int

	// EntriesChunkSize sets the maximum number of multihashes to include in a single entries chunk.
	// Defaults to 16384 if not specified. Note that chunks are chained together for indexing
	// advertisements that include more multihashes than the configured EntriesChunkSize.
	EntriesChunkSize int

	// TopicName sets the topic name on which the changes to the advertised content are announced.
	// If not explicitly specified, the topic name is automatically inferred from the network name
	// in following format: '/indexer/ingest/<network-name>'
	// Defaults to empty, which implies the topic name is inferred from network name.
	TopicName string

	// PurgeCacheOnStart sets whether to clear any cached entries chunks when the provider engine
	// starts. By default, the cache is rehydrated from previously cached entries stored in
	// datastore if any is present.
	PurgeCacheOnStart bool

	// The network indexer host that the web UI should link to for published announcements
	WebHost string

	Announce IndexProviderAnnounceConfig

	HttpPublisher IndexProviderHttpPublisherConfig

	// Set this to true to use the legacy data-transfer/graphsync publisher.
	// This should only be used as a temporary fall-back if publishing ipnisync
	// over libp2p or HTTP is not working, and publishing over
	// data-transfer/graphsync was previously working.
	DataTransferPublisher bool
}

type IndexProviderAnnounceConfig struct {
	// Make a direct announcement to a list of indexing nodes over http.
	// Note that announcements are already made over pubsub regardless
	// of this setting.
	AnnounceOverHttp bool

	// The list of URLs of indexing nodes to announce to.
	DirectAnnounceURLs []string
}

type IndexProviderHttpPublisherConfig struct {
	// If enabled, requests are served over HTTP instead of libp2p.
	Enabled bool
	// Set the public hostname / IP for the index provider listener.
	// eg "82.129.73.111"
	// This is usually the same as the for the boost node.
	PublicHostname string
	// Set the port on which to listen for index provider requests over HTTP.
	// Note that this port must be open on the firewall.
	Port int
	// Set this to true to publish HTTP over libp2p in addition to plain HTTP,
	// Otherwise, the publisher will publish content advertisements using only
	// plain HTTP if Enabled is true.
	WithLibp2p bool
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

	// The public multi-address for retrieving deals with droplet.
	// Note: Must be in multiaddr format, eg /ip4/127.0.0.1/tcp/41235/http
	HTTPRetrievalMultiaddr string

	IndexProvider IndexProviderConfig
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
		HTTPRetrievalMultiaddr: "",

		IndexProvider: IndexProviderConfig{
			Enable:               false,
			EntriesCacheCapacity: 1024,
			EntriesChunkSize:     16384,
			TopicName:            "",
			PurgeCacheOnStart:    false,
			WebHost:              "cid.contact",
			Announce: IndexProviderAnnounceConfig{
				AnnounceOverHttp:   false,
				DirectAnnounceURLs: []string{"https://cid.contact/ingest/announce"},
			},
			HttpPublisher: IndexProviderHttpPublisherConfig{
				Enabled:        false,
				PublicHostname: "",
				Port:           0,
				WithLibp2p:     false,
			},
			DataTransferPublisher: false,
		},
	}
}
