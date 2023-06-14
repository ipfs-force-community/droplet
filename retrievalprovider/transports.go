package retrievalprovider

import (
	"fmt"
	"time"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

// TransportsListener listens for incoming queries over libp2p
type TransportsListener struct {
	host      host.Host
	protocols []types.Protocol
}

func NewTransportsListener(h host.Host, cfg *config.MarketConfig) (*TransportsListener, error) {
	var protos []types.Protocol

	// Get the libp2p addresses from the Host
	if len(h.Addrs()) > 0 {
		protos = append(protos, types.Protocol{
			Name:      "libp2p",
			Addresses: h.Addrs(),
		})
	}

	// If there's an http retrieval address specified, add HTTP to the list
	// of supported protocols
	// todo: handle cfg.Miners[].HTTPRetrievalMultiaddr?
	if len(cfg.CommonProvider.HTTPRetrievalMultiaddr) != 0 {
		maddr, err := multiaddr.NewMultiaddr(cfg.CommonProvider.HTTPRetrievalMultiaddr)
		if err != nil {
			return nil, fmt.Errorf("could not parse '%s' as multiaddr: %w", cfg.CommonProvider.HTTPRetrievalMultiaddr, err)
		}

		protos = append(protos, types.Protocol{
			Name:      "http",
			Addresses: []multiaddr.Multiaddr{maddr},
		})
	}

	return &TransportsListener{
		host:      h,
		protocols: protos,
	}, nil
}

func (l *TransportsListener) Start() {
	l.host.SetStreamHandler(types.TransportsProtocolID, l.handleNewQueryStream)
}

func (l *TransportsListener) Stop() {
	l.host.RemoveStreamHandler(types.TransportsProtocolID)
}

// Called when the client opens a libp2p stream
func (l *TransportsListener) handleNewQueryStream(s network.Stream) {
	defer s.Close() // nolint

	log.Debugw("query", "peer", s.Conn().RemotePeer())

	response := types.QueryResponse{Protocols: l.protocols}

	// Set a deadline on writing to the stream so it doesn't hang
	_ = s.SetWriteDeadline(time.Now().Add(time.Second * 30))
	defer s.SetWriteDeadline(time.Time{}) // nolint

	// Write the response to the client
	err := types.BindnodeRegistry.TypeToWriter(&response, s, dagcbor.Encode)
	if err != nil {
		log.Infow("error writing query response", "peer", s.Conn().RemotePeer(), "err", err)
		return
	}
}
