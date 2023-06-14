package main

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
)

var dropletRepoFlag = &cli.StringFlag{
	Name:    "droplet-repo",
	Usage:   "droplet repo path, get the piece store configuration from it",
	Aliases: []string{"repo"},
	Value:   "~/.droplet",
}

var queryProtocols = &cli.Command{
	Name:  "protocols",
	Usage: "query retrieval support protocols",
	Flags: []cli.Flag{
		dropletRepoFlag,
	},
	ArgsUsage: "<miner>",
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() == 0 {
			return fmt.Errorf("must pass miner")
		}

		api, closer, err := cli2.NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := cli2.ReqContext(cctx)

		miner, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}
		minerInfo, err := api.StateMinerInfo(ctx, miner, types.EmptyTSK)
		if err != nil {
			return err
		}
		if minerInfo.PeerId == nil {
			return fmt.Errorf("peer id is nil")
		}

		h, err := libp2p.New(
			libp2p.Identity(nil),
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		)
		if err != nil {
			return err
		}

		addrs, err := utils.ConvertMultiaddr(minerInfo.Multiaddrs)
		if err != nil {
			return err
		}
		if err := h.Connect(ctx, peer.AddrInfo{ID: *minerInfo.PeerId, Addrs: addrs}); err != nil {
			return err
		}
		stream, err := h.NewStream(ctx, *minerInfo.PeerId, market.TransportsProtocolID)
		if err != nil {
			return fmt.Errorf("failed to open stream to peer: %w", err)
		}
		_ = stream.SetReadDeadline(time.Now().Add(time.Minute))
		//nolint: errcheck
		defer stream.SetReadDeadline(time.Time{})

		// Read the response from the stream
		queryResponsei, err := market.BindnodeRegistry.TypeFromReader(stream, (*market.QueryResponse)(nil), dagcbor.Decode)
		if err != nil {
			return fmt.Errorf("reading query response: %w", err)
		}
		queryResponse := queryResponsei.(*market.QueryResponse)

		for _, p := range queryResponse.Protocols {
			fmt.Println(p)
		}

		return nil
	},
}
