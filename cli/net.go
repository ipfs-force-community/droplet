package cli

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

var NetCmd = &cli.Command{
	Name:  "net",
	Usage: "Manage P2P Network",
	Subcommands: []*cli.Command{
		NetListen,
		NetId,
		NetConnect,
		NetPeers,
	},
}

var NetListen = &cli.Command{
	Name:  "listen",
	Usage: "List listen addresses",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		addrs, err := api.NetAddrsListen(ctx)
		if err != nil {
			return err
		}

		for _, peer := range addrs.Addrs {
			fmt.Printf("%s/p2p/%s\n", peer, addrs.ID)
		}
		return nil
	},
}

var NetId = &cli.Command{
	Name:  "id",
	Usage: "Get node identity",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		pid, err := api.ID(ctx)
		if err != nil {
			return err
		}

		fmt.Println(pid)
		return nil
	},
}

var NetConnect = &cli.Command{
	Name:  "connect",
	Usage: "connect to a indexer node",
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must and only input <peer-address> to connect")
		}
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		if err := api.NetConnect(ctx, cctx.Args().First()); err != nil {
			return err
		}

		fmt.Println("connect to peer successfully")
		return nil
	},
}

var NetPeers = &cli.Command{
	Name:  "peers",
	Usage: "list connected peers",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		addrs, err := api.NetPeers(ctx)
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			fmt.Printf("%s\n", addr.String())
		}

		return nil
	},
}
