package network

import (
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/types"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	"net"
	"time"

	"github.com/ipfs-force-community/venus-common-utils/metrics"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	blake2b "github.com/minio/blake2b-simd"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/fx"
)

func init() {
	// configure larger overlay parameters
	pubsub.GossipSubD = 8
	pubsub.GossipSubDscore = 6
	pubsub.GossipSubDout = 3
	pubsub.GossipSubDlo = 6
	pubsub.GossipSubDhi = 12
	pubsub.GossipSubDlazy = 12
	pubsub.GossipSubDirectConnectInitialDelay = 30 * time.Second
	pubsub.GossipSubIWantFollowupTime = 5 * time.Second
	pubsub.GossipSubHistoryLength = 10
	pubsub.GossipSubGossipFactor = 0.1
}

const (
	GossipScoreThreshold             = -500
	PublishScoreThreshold            = -1000
	GraylistScoreThreshold           = -2500
	AcceptPXScoreThreshold           = 1000
	OpportunisticGraftScoreThreshold = 3.5
)

type GossipIn struct {
	fx.In
	Mctx metrics.MetricsCtx
	Lc   fx.Lifecycle
	Host host.Host
	Nn   types2.NetworkName
	Sk   *types.ScoreKeeper `optional:"true"`
	Cfg  *config.Libp2p
}

func GossipSub(in GossipIn) (*pubsub.PubSub, error) {
	ingestTopicParams := &pubsub.TopicScoreParams{
		// expected ~0.5 confirmed deals / min. sampled
		TopicWeight: 0.1,

		TimeInMeshWeight:  0.00027, // ~1/3600
		TimeInMeshQuantum: time.Second,
		TimeInMeshCap:     1,

		FirstMessageDeliveriesWeight: 0.5,
		FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
		FirstMessageDeliveriesCap:    100, // allowing for burstiness

		InvalidMessageDeliveriesWeight: -1000,
		InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
	}

	topicParams := map[string]*pubsub.TopicScoreParams{}

	pgTopicWeights := map[string]float64{}

	// Index ingestion whitelist
	topicParams[IndexerIngestTopic(in.Nn)] = ingestTopicParams

	// IP colocation whitelist
	var ipcoloWhitelist []*net.IPNet

	if in.Sk == nil {
		in.Sk = new(types.ScoreKeeper)
	}
	options := []pubsub.Option{
		// Gossipsubv1.1 configuration
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageIdFn(HashMsgId),
		pubsub.WithPeerScore(
			&pubsub.PeerScoreParams{
				AppSpecificScore: func(p peer.ID) float64 {
					return 0
				},
				AppSpecificWeight: 1,

				// This sets the IP colocation threshold to 5 peers before we apply penalties
				IPColocationFactorThreshold: 5,
				IPColocationFactorWeight:    -100,
				IPColocationFactorWhitelist: ipcoloWhitelist,

				// P7: behavioural penalties, decay after 1hr
				BehaviourPenaltyThreshold: 6,
				BehaviourPenaltyWeight:    -10,
				BehaviourPenaltyDecay:     pubsub.ScoreParameterDecay(time.Hour),

				DecayInterval: pubsub.DefaultDecayInterval,
				DecayToZero:   pubsub.DefaultDecayToZero,

				// this retains non-positive scores for 6 hours
				RetainScore: 6 * time.Hour,

				// topic parameters
				Topics: topicParams,
			},
			&pubsub.PeerScoreThresholds{
				GossipThreshold:             GossipScoreThreshold,
				PublishThreshold:            PublishScoreThreshold,
				GraylistThreshold:           GraylistScoreThreshold,
				AcceptPXThreshold:           AcceptPXScoreThreshold,
				OpportunisticGraftThreshold: OpportunisticGraftScoreThreshold,
			},
		),
		pubsub.WithPeerScoreInspect(in.Sk.Update, 10*time.Second),
	}

	// direct peers
	if in.Cfg.ProtectedPeers != nil {
		var directPeerInfo []peer.AddrInfo
		for _, addr := range in.Cfg.ProtectedPeers {
			a, err := ma.NewMultiaddr(addr)
			if err != nil {
				return nil, err
			}

			pi, err := peer.AddrInfoFromP2pAddr(a)
			if err != nil {
				return nil, err
			}

			directPeerInfo = append(directPeerInfo, *pi)
		}

		options = append(options, pubsub.WithDirectPeers(directPeerInfo))
	}

	var pgParams = pubsub.NewPeerGaterParams(0.33,
		pubsub.ScoreParameterDecay(2*time.Minute),
		pubsub.ScoreParameterDecay(time.Hour),
	).WithTopicDeliveryWeights(pgTopicWeights)

	options = append(options, pubsub.WithPeerGater(pgParams))

	allowTopics := []string{IndexerIngestTopic(in.Nn)}

	options = append(options,
		pubsub.WithSubscriptionFilter(
			pubsub.WrapLimitSubscriptionFilter(
				pubsub.NewAllowlistSubscriptionFilter(allowTopics...),
				100)))

	return pubsub.NewGossipSub(metrics.LifecycleCtx(in.Mctx, in.Lc), in.Host, options...)
}

func HashMsgId(m *pubsub_pb.Message) string {
	hash := blake2b.Sum256(m.Data)
	return string(hash[:])
}

func IndexerIngestTopic(netName types2.NetworkName) string {
	nn := string(netName)
	// The network name testnetnet is here for historical reasons.
	// Going forward we aim to use the name `mainnet` where possible.
	if nn == "testnetnet" {
		nn = "mainnet"
	}

	return "/indexer/ingest/" + nn
}
