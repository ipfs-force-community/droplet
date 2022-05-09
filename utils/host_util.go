package utils

import (
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func HostConnectedPeers(h host.Host) []peer.AddrInfo {
	conns := h.Network().Conns()
	infos := make([]peer.AddrInfo, len(conns))

	for i, conn := range conns {
		infos[i] = peer.AddrInfo{
			ID: conn.RemotePeer(),
			Addrs: []ma.Multiaddr{
				conn.RemoteMultiaddr(),
			},
		}
	}
	return infos
}
