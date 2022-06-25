package cli

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	miner7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/v8/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var ActorCmd = &cli.Command{
	Name:  "actor",
	Usage: "manipulate the miner actor",
	Subcommands: []*cli.Command{
		actorListCmd,
		actorSetAddrsCmd,
		actorSetPeeridCmd,
		actorInfoCmd,
	},
}

var actorListCmd = &cli.Command{
	Name:  "list",
	Usage: "find miners in the system",
	Flags: []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		miners, err := nodeAPI.ActorList(cctx.Context)
		if err != nil {
			return err
		}

		buf := &bytes.Buffer{}
		tw := tabwriter.NewWriter(buf, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "miner\taccount")
		for _, miner := range miners {
			_, _ = fmt.Fprintf(tw, "%s\t%s\n", miner.Addr.String(), miner.Account)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Println(buf.String())

		return nil
	},
}

var actorSetAddrsCmd = &cli.Command{
	Name:  "set-addrs",
	Usage: "set addresses that your miner can be publicly dialed on",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "gas-limit",
			Usage: "set gas limit",
			Value: 0,
		},
		&cli.BoolFlag{
			Name:  "unset",
			Usage: "unset address",
			Value: false,
		},
		&cli.StringFlag{
			Name:     "miner",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		args := cctx.Args().Slice()
		unset := cctx.Bool("unset")
		if len(args) == 0 && !unset {
			return cli.ShowSubcommandHelp(cctx)
		}
		if len(args) > 0 && unset {
			return fmt.Errorf("unset can only be used with no arguments")
		}

		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		api, acloser, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer acloser()

		ctx := ReqContext(cctx)

		var addrs []abi.Multiaddrs
		for _, a := range args {
			maddr, err := ma.NewMultiaddr(a)
			if err != nil {
				return fmt.Errorf("failed to parse %q as a multiaddr: %w", a, err)
			}

			maddrNop2p, strip := ma.SplitFunc(maddr, func(c ma.Component) bool {
				return c.Protocol().Code == ma.P_P2P
			})

			if strip != nil {
				fmt.Println("Stripping peerid ", strip, " from ", maddr)
			}
			addrs = append(addrs, maddrNop2p.Bytes())
		}

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return nil
		}

		if bHas, _ := nodeAPI.ActorExist(ctx, maddr); !bHas {
			return fmt.Errorf("actor [%s] not found", maddr)
		}

		minfo, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		params, err := actors.SerializeParams(&miner7.ChangeMultiaddrsParams{NewMultiaddrs: addrs})
		if err != nil {
			return err
		}

		gasLimit := cctx.Int64("gas-limit")

		mid, err := nodeAPI.MessagerPushMessage(ctx, &types.Message{
			To:       maddr,
			From:     minfo.Worker,
			Value:    types.NewInt(0),
			GasLimit: gasLimit,
			Method:   builtin.MethodsMiner.ChangeMultiaddrs,
			Params:   params,
		}, nil)
		if err != nil {
			return err
		}

		fmt.Printf("Requested multiaddrs change in message %s\n", mid)
		return nil

	},
}

var actorSetPeeridCmd = &cli.Command{
	Name:  "set-peer-id",
	Usage: "set the peer id of your miner",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "gas-limit",
			Usage: "set gas limit",
			Value: 0,
		},
		&cli.StringFlag{
			Name:     "miner",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		api, acloser, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer acloser()

		ctx := ReqContext(cctx)

		pid, err := peer.Decode(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("failed to parse input as a peerId: %w", err)
		}

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return nil
		}

		if bHas, _ := nodeAPI.ActorExist(ctx, maddr); !bHas {
			return fmt.Errorf("actor [%s] not found", maddr)
		}

		minfo, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		params, err := actors.SerializeParams(&miner7.ChangePeerIDParams{NewID: abi.PeerID(pid)})
		if err != nil {
			return err
		}

		gasLimit := cctx.Int64("gas-limit")

		mid, err := nodeAPI.MessagerPushMessage(ctx, &types.Message{
			To:       maddr,
			From:     minfo.Worker,
			Value:    types.NewInt(0),
			GasLimit: gasLimit,
			Method:   builtin.MethodsMiner.ChangePeerID,
			Params:   params,
		}, nil)
		if err != nil {
			return err
		}

		fmt.Printf("Requested peerid change in message %s\n", mid)
		return nil

	},
}

var actorInfoCmd = &cli.Command{
	Name:  "info",
	Usage: "query info of specified miner",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "miner",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		api, acloser, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer acloser()

		ctx := ReqContext(cctx)

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return nil
		}

		if bHas, _ := nodeAPI.ActorExist(ctx, maddr); !bHas {
			return fmt.Errorf("actor [%s] not found", maddr)
		}

		minfo, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		peerIdStr := ""
		if minfo.PeerId != nil {
			peerIdStr = minfo.PeerId.String()
		}
		fmt.Println("peers:", peerIdStr)
		fmt.Println("addr:")
		for _, addrBytes := range minfo.Multiaddrs {
			addr, err := ma.NewMultiaddrBytes(addrBytes)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println("\t", addr.String())
		}

		return nil
	},
}
