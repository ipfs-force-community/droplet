package main

import (
	"fmt"

	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/cli/tablewriter"
	"github.com/urfave/cli/v2"
)

var pieceInfoCommands = &cli.Command{
	Name:        "piece-info",
	Usage:       "Store piece cid, piece size, payload cid, payload size to the database for easy use in direct deal",
	Subcommands: []*cli.Command{importPieceInfoCmd, listPieceInfoCmd},
}

var importPieceInfoCmd = &cli.Command{
	Name:  "import",
	Usage: "import piece info to database",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "manifest",
			Usage:    "Path to the manifest file",
			Required: true,
		},
	},
	Action: func(cliCtx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cliCtx)

		manifests, err := loadManifest(cliCtx.String("manifest"))
		if err != nil {
			return fmt.Errorf("load manifest error: %v", err)
		}

		pis := make([]*types.ClientPieceInfo, 0, len(manifests))
		for _, val := range manifests {
			pi := &types.ClientPieceInfo{
				PieceCID:    val.pieceCID,
				PieceSize:   val.payloadSize,
				PayloadCID:  val.payloadCID,
				PayloadSize: val.payloadSize,
			}
			pis = append(pis, pi)
		}

		return api.ClientImportPieceInfos(ctx, pis)
	},
}

var listPieceInfoCmd = &cli.Command{
	Name:  "list",
	Usage: "list piece info",
	Action: func(cliCtx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cliCtx)

		pis, err := api.ClientListPieceInfo(ctx)
		if err != nil {
			return err
		}

		w := tablewriter.New(tablewriter.Col("ID"),
			tablewriter.Col("PieceCID"),
			tablewriter.Col("PieceSize"),
			tablewriter.Col("PayloadCID"),
			tablewriter.Col("PayloadSize"),
		)
		for _, pi := range pis {
			w.Write(map[string]interface{}{
				"ID":          pi.ID,
				"PieceCID":    pi.PieceCID,
				"PieceSize":   pi.PayloadSize,
				"PayloadCID":  pi.PayloadCID,
				"PayloadSize": pi.PayloadSize,
			})
		}

		return w.Flush(cliCtx.App.Writer)
	},
}
