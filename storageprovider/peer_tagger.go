package storageprovider

import (
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type PeerTagger struct {
	net network.StorageMarketNetwork
}

func newPeerTagger(net network.StorageMarketNetwork) *PeerTagger {
	return &PeerTagger{net: net}
}

func (p *PeerTagger) TagPeer(id peer.ID, s string) {
	p.net.TagPeer(id, s)
}

func (p *PeerTagger) UntagPeer(id peer.ID, s string) {
	p.net.UntagPeer(id, s)
}
