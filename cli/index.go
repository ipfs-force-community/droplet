package cli

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

var IndexCmds = &cli.Command{
	Name:  "index",
	Usage: "index management",
	Subcommands: []*cli.Command{
		announceCmd,
	},
}

var announceCmd = &cli.Command{
	Name:  "announce",
	Usage: "republish atest advertisement",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)
		advCid, err := api.Announce(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("publish latest advertisement:%s successfully\n", advCid.String())
		return nil
	},
}