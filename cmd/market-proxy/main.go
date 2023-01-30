package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"

	"github.com/filecoin-project/venus-market/v2/config"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-graphsync/network"

	"github.com/filecoin-project/venus-market/v2/protocolproxy"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/filecoin-project/venus-market/v2/version"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var protocols = []protocol.ID{
	//storage
	storagemarket.AskProtocolID,
	storagemarket.OldAskProtocolID,

	storagemarket.DealProtocolID110,
	storagemarket.DealProtocolID101,
	storagemarket.DealProtocolID111,

	storagemarket.OldDealStatusProtocolID,
	storagemarket.DealStatusProtocolID,

	//retrieval
	retrievalmarket.QueryProtocolID,
	retrievalmarket.OldQueryProtocolID,

	network.ProtocolGraphsync_1_0_0,
	network.ProtocolGraphsync_2_0_0,
}

var mainLog = logging.Logger("market-proxy")

func main() {
	app := &cli.App{
		Name:                 "venus-market",
		Usage:                "venus-market",
		Version:              version.UserVersion(),
		EnableBashCompletion: true,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type ProxyConfig struct {
	ProxyServer string
}

func run(cfg *config.ProxyServer) error {
	var pkey crypto.PrivKey
	var err error
	if len(cfg.PrivateKey) == 0 {
		privateKeyBytes, err := hex.DecodeString(cfg.PrivateKey)
		if err != nil {
			return err
		}
		crypto.UnmarshalPrivateKey(privateKeyBytes)
	} else {
		pkey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return err
		}
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(cfg.ListenAddresses...),
		libp2p.Identity(pkey),
		libp2p.DefaultPeerstore,
		libp2p.NoListenAddrs,
		libp2p.Ping(true),
		libp2p.UserAgent("venus-market-proxy" + version.UserVersion()),
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return err
	}

	addrInfo, err := peer.AddrInfoFromString(cfg.Backends[0])
	if err != nil {
		return err
	}
	proxyHost, err := protocolproxy.NewProtocolProxy(h, map[peer.ID][]protocol.ID{
		addrInfo.ID: protocols,
	})
	if err != nil {
		return err
	}
	defer proxyHost.Close()

	proxyHost.Start(context.Background())
	return nil
}
