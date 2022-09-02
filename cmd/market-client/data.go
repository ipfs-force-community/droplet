package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"

	cli2 "github.com/filecoin-project/venus-market/v2/cli"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
)

var dataCmd = &cli.Command{
	Name:        "data",
	Usage:       "auxiliary file conversion",
	Description: `Convert local files or data to Filecoin network acceptable types.`,
	Subcommands: []*cli.Command{
		dataImportCmd,
		dataDropCmd,
		dataLocalCmd,
		dataStatCmd,
		dataCommPCmd,
		dataGenerateCarCmd,
	},
}

var dataImportCmd = &cli.Command{
	Name:      "import",
	Usage:     "Import data",
	ArgsUsage: "[inputPath]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "car",
			Usage: "import from a car file instead of a regular file",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Output root CID only",
		},
		&cli2.CidBaseFlag,
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		if cctx.NArg() != 1 {
			return errors.New("expected input path as the only arg")
		}

		absPath, err := filepath.Abs(cctx.Args().First())
		if err != nil {
			return err
		}

		ref := client.FileRef{
			Path:  absPath,
			IsCAR: cctx.Bool("car"),
		}
		c, err := api.ClientImport(ctx, ref)
		if err != nil {
			return err
		}

		encoder, err := cli2.GetCidEncoder(cctx)
		if err != nil {
			return err
		}

		if !cctx.Bool("quiet") {
			fmt.Printf("Import %d, Root ", c.ImportID)
		}
		fmt.Println(encoder.Encode(c.Root))

		return nil
	},
}

var dataDropCmd = &cli.Command{
	Name:      "drop",
	Usage:     "Remove import",
	ArgsUsage: "[import ID...]",
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return fmt.Errorf("no imports specified")
		}

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		var ids []uint64
		for i, s := range cctx.Args().Slice() {
			id, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing %d-th import ID: %w", i, err)
			}

			ids = append(ids, id)
		}

		for _, id := range ids {
			if err := api.ClientRemoveImport(ctx, client.ImportID(id)); err != nil {
				return fmt.Errorf("removing import %d: %w", id, err)
			}
		}

		return nil
	},
}

var dataLocalCmd = &cli.Command{
	Name:  "local",
	Usage: "List locally imported data",
	Flags: []cli.Flag{
		&cli2.CidBaseFlag,
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		list, err := api.ClientListImports(ctx)
		if err != nil {
			return err
		}

		encoder, err := cli2.GetCidEncoder(cctx)
		if err != nil {
			return err
		}

		sort.Slice(list, func(i, j int) bool {
			return list[i].Key < list[j].Key
		})

		for _, v := range list {
			cidStr := "<nil>"
			if v.Root != nil {
				cidStr = encoder.Encode(*v.Root)
			}

			fmt.Printf("%d: %s @%s (%s)\n", v.Key, cidStr, v.FilePath, v.Source)
			if v.Err != "" {
				fmt.Printf("\terror: %s\n", v.Err)
			}
		}
		return nil
	},
}

var dataStatCmd = &cli.Command{
	Name:      "stat",
	Usage:     "Print information about a locally stored file (piece size, etc)",
	ArgsUsage: "<cid>",
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		if !cctx.Args().Present() || cctx.NArg() != 1 {
			return fmt.Errorf("must specify cid of data")
		}

		dataCid, err := cid.Parse(cctx.Args().First())
		if err != nil {
			return fmt.Errorf("parsing data cid: %w", err)
		}

		ds, err := api.ClientDealSize(ctx, dataCid)
		if err != nil {
			return err
		}

		fmt.Printf("Piece Size  : %v\n", ds.PieceSize)
		fmt.Printf("Payload Size: %v\n", ds.PayloadSize)

		return nil
	},
}

var dataCommPCmd = &cli.Command{
	Name:      "commP",
	Usage:     "Calculate the piece-cid (commP) of a CAR file",
	ArgsUsage: "[inputFile]",
	Flags: []cli.Flag{
		&cli2.CidBaseFlag,
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		if cctx.Args().Len() != 1 {
			return fmt.Errorf("usage: commP <inputPath>")
		}

		ret, err := api.ClientCalcCommP(ctx, cctx.Args().Get(0))
		if err != nil {
			return err
		}

		encoder, err := cli2.GetCidEncoder(cctx)
		if err != nil {
			return err
		}

		fmt.Println("CID: ", encoder.Encode(ret.Root))
		fmt.Printf("Piece size: %s ( %d B )\n", types.SizeStr(types.NewInt(uint64(ret.Size))), ret.Size)
		return nil
	},
}

var dataGenerateCarCmd = &cli.Command{
	Name:      "generate-car",
	Usage:     "Generate a car file from input",
	ArgsUsage: "[inputPath outputPath]",
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		if cctx.Args().Len() != 2 {
			return fmt.Errorf("usage: generate-car <inputPath> <outputPath>")
		}

		ref := client.FileRef{
			Path:  cctx.Args().First(),
			IsCAR: false,
		}
		op := cctx.Args().Get(1)
		return api.ClientGenCar(ctx, ref, op)
	},
}
