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
		pieceStorageAddFsCmd,
		pieceStorageAddS3Cmd,
		pieceStorageListCmd,
		pieceStorageRemoveCmd,
	},
}

var pieceStorageAddFsCmd = &cli.Command{
	Name:  "add-fs",
	Usage: "add a local filesystem piece storage",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "path to the filesystem piece storage",
		},
		&cli.BoolFlag{
			Name:    "read-only",
			Aliases: []string{"r"},
			Usage:   "read-only filesystem piece storage",
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

		if !cctx.IsSet("path") {
			return fmt.Errorf("path is required")
		}
		if !cctx.IsSet("name") {
			return fmt.Errorf("name is required")
		}

		ctx := ReqContext(cctx)
		path := cctx.String("path")
		readOnly := cctx.Bool("read-only")
		name := cctx.String("name")

		err = nodeApi.AddFsPieceStorage(ctx, name, path, readOnly)
		if err != nil {
			return err
		}
		fmt.Println("Adding filesystem piece storage:", path)

		return nil
	},
}

var pieceStorageAddS3Cmd = &cli.Command{
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
			Name:     "endpoint",
			Aliases:  []string{"e"},
			Usage:    "endpoint of the S3 storage provider",
			Required: true,
		},
		// name
		&cli.StringFlag{
			Name:     "name",
			Aliases:  []string{"n"},
			Usage:    "name of the S3 piece storage",
			Required: true,
		},
		// bucket
		&cli.StringFlag{
			Name:     "bucket",
			Aliases:  []string{"b"},
			Usage:    "name of the S3 bucket",
			Required: true,
		},
		// dir
		&cli.StringFlag{
			Name:    "subdir",
			Aliases: []string{"d"},
			Usage:   "subdirectory of the S3 bucket to store the pieces in. ",
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		// get access key , secret key ,token interactivelly
		getS3Credentials := func() (string, string, string, error) {
			// var accessKey, secretKey, token string
			afmt := NewAppFmt(cctx.App)

			accessKey, err := afmt.GetScret("access key:", true)
			if err != nil {
				return "", "", "", err
			}

			secretKey, err := afmt.GetScret("secret key:", true)
			if err != nil {
				return "", "", "", err
			}

			token, err := afmt.GetScret("token:", true)
			if err != nil {
				return "", "", "", err
			}

			return accessKey, secretKey, token, err
		}

		accessKey, secretKey, token, err := getS3Credentials()
		if err != nil {
			return err
		}
		readOnly := cctx.Bool("readonly")
		endpoint := cctx.String("endpoint")
		name := cctx.String("name")
		bucket := cctx.String("bucket")
		subdir := cctx.String("subdir")

		err = nodeApi.AddS3PieceStorage(ctx, name, endpoint, bucket, subdir, accessKey, secretKey, token, readOnly)
		if err != nil {
			return err
		}
		fmt.Println("Adding S3 piece storage:", endpoint)

		return nil
	},
}

var pieceStorageListCmd = &cli.Command{
	Name:  "list",
	Usage: "list piece storages",
	Action: func(cctx *cli.Context) error {
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		storagelist := nodeApi.ListPieceStorageInfos(ctx)

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
				"Name":     storage.Name,
				"Readonly": storage.ReadOnly,
				"Path":     storage.EndPoint + "/" + storage.SubDir + storage.Bucket,
				"Type":     "S3",
			})
		}

		return w.Flush(os.Stdout)
	},
}

var pieceStorageRemoveCmd = &cli.Command{
	Name:      "remove",
	ArgsUsage: "<name>",
	Usage:     "remove a piece storage",
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
