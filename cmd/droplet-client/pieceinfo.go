package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	"github.com/urfave/cli/v2"
)

var pieceInfoCmds = &cli.Command{
	Name:  "piece-info",
	Usage: "",
	Subcommands: []*cli.Command{
		generateManifestFromPieceFileCmd,
	},
}

var generateManifestFromPieceFileCmd = &cli.Command{
	Name:  "gen-manifest",
	Usage: "generate manifest from piece file",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "output",
			Value: "./manifest.csv",
		},
		&cli.StringSliceFlag{
			Name:  "skip",
			Usage: "skip piece file, eg --skip xxxx1 --skip xxxx2",
		},
		&cli.BoolFlag{
			Name:  "is-padding",
			Usage: "Whether the car file is padding",
		},
	},
	ArgsUsage: "<piece-dir>",
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.Args().Len() < 1 {
			return fmt.Errorf("mus pass piece directory")
		}
		dir := cliCtx.Args().First()
		output := cliCtx.String("output")

		skips := make(map[string]struct{})
		for _, piece := range cliCtx.StringSlice("skip") {
			skips[piece] = struct{}{}
		}

		isPadding := cliCtx.Bool("is-padding")

		ms := make([]*manifest, 0)
		err := filepath.Walk(dir, func(path string, d fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			m, err := walkFile(path, d, skips, isPadding)
			if err != nil {
				fmt.Printf("walk %s failed: %v\n", path, err)
				return nil
			}
			if m != nil {
				ms = append(ms, m)
			}
			return nil
		})
		if err != nil {
			return err
		}

		buf := &bytes.Buffer{}
		writer := csv.NewWriter(buf)
		if err := writer.Write(strings.Split("payload_cid,piece_cid,payload_size,piece_size", ",")); err != nil {
			return err
		}
		for _, m := range ms {
			if err := writer.Write([]string{m.payloadCID.String(), m.pieceCID.String(), strconv.FormatUint(m.payloadSize, 10), strconv.FormatUint(uint64(m.pieceSize), 10)}); err != nil {
				return err
			}
		}
		writer.Flush()

		return os.WriteFile(output, buf.Bytes(), 0o755)
	},
}

func walkFile(path string, d fs.FileInfo, skips map[string]struct{}, isPadding bool) (*manifest, error) {
	name := d.Name()
	if _, ok := skips[name]; ok {
		fmt.Println("skip file:", name)
		return nil, nil
	}
	// xxxx.car
	if strings.Contains(name, ".car") {
		name = strings.TrimSuffix(name, ".car")
	}

	pieceCid, err := cid.Parse(name)
	if err != nil {
		return nil, fmt.Errorf("parse %s to cid failed: %v", name, err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	hd, err := car.ReadHeader(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("failed to read CAR header: %w", err)
	}
	if len(hd.Roots) != 1 {
		return nil, fmt.Errorf("car file can have one and only one header")
	}
	if hd.Version != 1 && hd.Version != 2 {
		return nil, fmt.Errorf("car version must be 1 or 2, is %d", hd.Version)
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()

	pieceSize := padreader.PaddedSize(uint64(size))
	if isPadding {
		pieceSize = abi.UnpaddedPieceSize(size)
	}

	return &manifest{
		payloadCID:  hd.Roots[0],
		payloadSize: uint64(size),
		pieceCID:    pieceCid,
		pieceSize:   pieceSize,
	}, nil
}
