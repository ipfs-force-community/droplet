package network

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"

	"github.com/ipfs-force-community/venus-common-utils/builder"
)

//nolint:golint
var (
	DefaultTransportsKey = builder.Special{ID: 0} // Libp2p option
	DiscoveryHandlerKey  = builder.Special{ID: 2} // Private type
	AddrsFactoryKey      = builder.Special{ID: 3} // Libp2p option
	SmuxTransportKey     = builder.Special{ID: 4} // Libp2p option
	RelayKey             = builder.Special{ID: 5} // Libp2p option
	SecurityKey          = builder.Special{ID: 6} // Libp2p option
)

// Invokes are called in the order they are defined.
var (
	PstoreAddSelfKeysKey = builder.NextInvoke()
	StartListeningKey    = builder.NextInvoke()
)

var NetworkOpts = func(server bool, simultaneousTransfers uint64) builder.Option {
	opts := builder.Options(
		builder.Override(new(host.Host), Host),
		//libp2p
		builder.Override(new(crypto.PrivKey), PrivKey),
		builder.Override(new(crypto.PubKey), crypto.PrivKey.GetPublic),
		builder.Override(new(peer.ID), peer.IDFromPublicKey),
		builder.Override(new(peerstore.Peerstore), pstoremem.NewPeerstore),
		builder.Override(PstoreAddSelfKeysKey, PstoreAddSelfKeys),
		builder.Override(StartListeningKey, StartListening),
		builder.Override(AddrsFactoryKey, AddrsFactory),
		builder.Override(DefaultTransportsKey, DefaultTransports),
		builder.Override(SmuxTransportKey, SmuxTransport(true)),
		builder.Override(RelayKey, NoRelay()),
		builder.Override(SecurityKey, Security(true, false)),
	)
	if server {
		return builder.Options(opts,
			builder.Override(new(StagingGraphsync), NewStagingGraphsync(simultaneousTransfers)),
		)
	} else {
		return builder.Options(opts,
			builder.Override(new(Graphsync), NewGraphsync(simultaneousTransfers)),
		)
	}
}
