package cli

import (
	"github.com/urfave/cli/v2"
)

var MigrateCmd = &cli.Command{
	Name:  "migrate",
	Usage: "Manage P2P Network",
	Subcommands: []*cli.Command{
		ImportV1DataCmd,
	},
}

var ImportV1DataCmd = &cli.Command{
	Name:  "import_v1",
	Usage: "import v1 data",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)
		err = api.ImportV1Data(ctx, cctx.Args().Get(0))
		if err != nil {
			return err
		}
		return nil
	},
}
