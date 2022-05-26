package network

import (
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

//nolint:golint
var (
	DefaultTransportsKey = builder.Special{ID: 0} // Libp2p option
	DiscoveryHandlerKey  = builder.Special{ID: 2} // Private type
	AddrsFactoryKey      = builder.Special{ID: 3} // Libp2p option
	SmuxTransportKey     = builder.Special{ID: 4} // Libp2p option
	RelayKey             = builder.Special{ID: 5} // Libp2p option
	SecurityKey          = builder.Special{ID: 6} // Libp2p option
	ResourceManagerKey   = builder.Special{ID: 7} // Libp2p option
)

// Invokes are called in the order they are defined.
var (
	PstoreAddSelfKeysKey = builder.NextInvoke()
	StartListeningKey    = builder.NextInvoke()
)

var NetworkOpts = func(server bool, simultaneousTransfersForRetrieval, simultaneousTransfersForStoragePerClient, simultaneousTransfersForStorage uint64) builder.Option {
	opts := builder.Options(
		builder.Override(new(host.Host), Host),
		//libp2p
		builder.Override(new(crypto.PrivKey), PrivKey),
		builder.Override(new(crypto.PubKey), crypto.PrivKey.GetPublic),
		builder.Override(new(peer.ID), peer.IDFromPublicKey),
		builder.Override(new(peerstore.Peerstore), NewPeerstore),
		builder.Override(PstoreAddSelfKeysKey, PstoreAddSelfKeys),
		builder.Override(StartListeningKey, StartListening),
		builder.Override(AddrsFactoryKey, AddrsFactory),
		builder.Override(DefaultTransportsKey, DefaultTransports),
		builder.Override(SmuxTransportKey, SmuxTransport()),
		builder.Override(RelayKey, NoRelay()),
		builder.Override(SecurityKey, Security(true, false)),
		builder.Override(new(network.ResourceManager), ResourceManager),
		builder.Override(ResourceManagerKey, ResourceManagerOption),
	)
	if server {
		return builder.Options(opts,
			builder.Override(new(StagingGraphsync), NewStagingGraphsync(simultaneousTransfersForRetrieval, simultaneousTransfersForStoragePerClient, simultaneousTransfersForStorage)),
		)
	}

	return builder.Options(opts,
		// retrieval/storage reverse for server/client
		builder.Override(new(Graphsync), NewGraphsync(simultaneousTransfersForRetrieval, simultaneousTransfersForStorage)),
	)
}
