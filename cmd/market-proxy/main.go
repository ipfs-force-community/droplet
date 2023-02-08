package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer"

	network2 "github.com/filecoin-project/venus-market/v2/network"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/utils"

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
	datatransfer.ProtocolDataTransfer1_2,
}

var mainLog = logging.Logger("market-proxy")

func main() {
	app := &cli.App{
		Name:                 "venus-market-proxy",
		Usage:                "proxy multiple venus-market backends like nginx",
		Version:              version.UserVersion(),
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "start a libp2p proxy",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "listen",
						Usage: "specify listen address ",
						Value: "/ip4/0.0.0.0/tcp/11023",
					},
					&cli.StringFlag{
						Name:  "peer-key",
						Usage: "peer key for p2p identify, if not specify, will generate new one",
						Value: "",
					},
					&cli.StringSliceFlag{
						Name:     "backends",
						Usage:    "a group of backends libp2p backends server",
						Required: true,
					},
				},
				Action: run,
			},
			{
				Name:   "new-peer-key",
				Usage:  "generate random peer key and corresponding private key ",
				Flags:  []cli.Flag{},
				Action: genKey,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	ctx := c.Context
	utils.SetupLogLevels()
	cfg := config.ProxyServer{}
	cfg.Libp2p.ListenAddresses = []string{c.String("listen")}
	cfg.Libp2p.PrivateKey = c.String("peer-key")
	cfg.Backends = c.StringSlice("backends")

	var pkey crypto.PrivKey
	var err error
	if len(cfg.PrivateKey) > 0 {
		privateKeyBytes, err := hex.DecodeString(cfg.PrivateKey)
		if err != nil {
			return err
		}
		pkey, err = crypto.UnmarshalPrivateKey(privateKeyBytes)
		if err != nil {
			return err
		}
	} else {
		pkey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return err
		}

		privateKeyBytes, err := crypto.MarshalPrivateKey(pkey)
		if err != nil {
			return err
		}

		fmt.Println(hex.EncodeToString(privateKeyBytes))
	}

	opts := []libp2p.Option{
		network2.MakeSmuxTransportOption(),
		libp2p.DefaultTransports,
		libp2p.ListenAddrStrings(cfg.ListenAddresses...),
		libp2p.Identity(pkey),
		libp2p.WithDialTimeout(time.Second * 5),
		libp2p.DefaultPeerstore,
		libp2p.DisableRelay(),
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

	//try to connect backends
	go func() {
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			err = h.Connect(ctx, *addrInfo)
			if err != nil {
				mainLog.Errorf("connect to %s %v", addrInfo, err)
			}
			<-timer.C
		}
	}()

	var libp2pAddrs []string
	for _, peer := range h.Addrs() {
		libp2pAddrs = append(libp2pAddrs, fmt.Sprintf("%s/p2p/%s\n", peer, h.ID()))
	}

	mainLog.Infof("start listen at %v", libp2pAddrs)
	shutdownChan := make(chan struct{})
	finishCh := utils.MonitorShutdown(shutdownChan)
	<-finishCh
	return nil
}

func genKey(c *cli.Context) error {
	pkey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return err
	}

	privateKeyBytes, err := crypto.MarshalPrivateKey(pkey)
	if err != nil {
		return err
	}
	peerId, err := peer.IDFromPrivateKey(pkey)
	if err != nil {
		return err
	}
	fmt.Println("PeerId:", peerId)
	fmt.Println("Pkey:", hex.EncodeToString(privateKeyBytes))
	return nil
}
