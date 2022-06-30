package cli

import (
	"fmt"
	"os"

	"github.com/filecoin-project/venus-market/v2/cli/tablewriter"
	"github.com/urfave/cli/v2"
)

var PieceStorageCmd = &cli.Command{
	Name:        "piece-storage",
	Usage:       "Manage piece storage ",
	Description: "The piece storage will decide where to store pieces and how to store them",
	Subcommands: []*cli.Command{
		addFsPieceStorageCmd,
		addS3PieceStorageCmd,
		PieceStorageListCmd,
		PieceStorageRemoveCmd,
	},
}

var addFsPieceStorageCmd = &cli.Command{
	Name:  "add-fs",
	Usage: "add a local filesystem piece storage",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "path to the filesystem piece storage",
		},
		&cli.BoolFlag{
			Name:        "read-only",
			Aliases:     []string{"r"},
			Usage:       "read-only filesystem piece storage",
			DefaultText: "false",
		},
		// name
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Usage:   "name of the filesystem piece storage",
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)
		path := cctx.String("path")
		readOnly := cctx.Bool("read-only")
		name := cctx.String("name")

		if path == "" {
			return fmt.Errorf("path is required")
		}
		if name == "" {
			return fmt.Errorf("name is required")
		}

		err = nodeApi.AddFsPieceStorage(ctx, readOnly, path, name)
		if err != nil {
			return err
		}
		fmt.Println("Adding filesystem piece storage:", path)

		return nil
	},
}

var addS3PieceStorageCmd = &cli.Command{
	Name:  "add-s3",
	Usage: "add a object storage for piece storage",
	Flags: []cli.Flag{
		// read only
		&cli.BoolFlag{
			Name:        "readonly",
			Aliases:     []string{"r"},
			Usage:       "set true if you want the piece storage only fro reading",
			DefaultText: "false",
		},
		// Endpoint
		&cli.StringFlag{
			Name:    "endpoint",
			Aliases: []string{"e"},
			Usage:   "endpoint of the S3 bucket",
		},
		// access key
		&cli.StringFlag{
			Name:    "access-key",
			Aliases: []string{"a"},
			Usage:   "access key of the S3 bucket",
		},
		// secret key
		&cli.StringFlag{
			Name:    "secret-key",
			Aliases: []string{"s"},
			Usage:   "secret key of the S3 bucket",
		},
		// token
		&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"t"},
			Usage:   "token of the S3 bucket",
		},
		// name
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Usage:   "name of the S3 bucket",
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		readOnly := cctx.Bool("readonly")
		endpoint := cctx.String("endpoint")
		accessKey := cctx.String("access-key")
		secretKey := cctx.String("secret-key")
		token := cctx.String("token")
		name := cctx.String("name")

		if endpoint == "" {
			return fmt.Errorf("endpoint are required")
		}
		if accessKey == "" {
			return fmt.Errorf("access key are required")
		}
		if secretKey == "" {
			return fmt.Errorf("secret key are required")
		}
		if name == "" {
			return fmt.Errorf("name are required")
		}

		err = nodeApi.AddS3PieceStorage(ctx, readOnly, endpoint, name, accessKey, secretKey, token)
		if err != nil {
			return err
		}
		fmt.Println("Adding S3 piece storage:", endpoint)

		return nil
	},
}

var PieceStorageListCmd = &cli.Command{
	Name:  "list",
	Usage: "list piece storages",
	Action: func(cctx *cli.Context) error {
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		storagelist := nodeApi.GetPieceStorages(ctx)

		w := tablewriter.New(
			tablewriter.Col("name"),
			tablewriter.Col("readonly"),
			tablewriter.Col("path/enter point"),
			tablewriter.Col("type"),
		)

		for _, storage := range storagelist.FsStorage {

			w.Write(map[string]interface{}{
				"Name":                storage.Name,
				"Readonly":            storage.ReadOnly,
				"Path or Enter point": storage.Path,
				"Type":                "file system",
			})
		}

		for _, storage := range storagelist.S3Storage {
			w.Write(map[string]interface{}{
				"Name":                storage.Name,
				"Readonly":            storage.ReadOnly,
				"Path or Enter point": storage.EndPoint,
				"Type":                "S3",
			})
		}

		w.Flush(os.Stdout)

		return nil
	},
}

var PieceStorageRemoveCmd = &cli.Command{
	Name:  "remove",
	Usage: "remove a piece storage",
	Action: func(cctx *cli.Context) error {
		// get idx
		name := cctx.Args().Get(0)
		if name == "" {
			return fmt.Errorf("piece storage name is required")
		}

		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)
		return nodeApi.RemovePieceStorage(ctx, name)
	},
}
