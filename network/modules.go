package network

import (
	"github.com/filecoin-project/venus-market/builder"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
)

//nolint:golint
var (
	DefaultTransportsKey = builder.Special{0} // Libp2p option
	DiscoveryHandlerKey  = builder.Special{2} // Private type
	AddrsFactoryKey      = builder.Special{3} // Libp2p option
	SmuxTransportKey     = builder.Special{4} // Libp2p option
	RelayKey             = builder.Special{5} // Libp2p option
	SecurityKey          = builder.Special{6} // Libp2p option
)

const (
	PstoreAddSelfKeysKey builder.Invoke = 0
	StartListeningKey    builder.Invoke = 1
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
