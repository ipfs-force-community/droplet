package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/docker/go-units"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var storageAsksCmds = &cli.Command{
	Name:  "ask",
	Usage: "Configure storage asks",
	Subcommands: []*cli.Command{
		setAskCmd,
		getAskCmd,
	},
}

var setAskCmd = &cli.Command{
	Name:      "set",
	ArgsUsage: "<miner address>",
	Usage:     "Configure(set/update) the miner's ask",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "price",
			Usage: "Set the price of the ask for unverified deals (specified as FIL / GiB / Epoch) to `PRICE`.",
			Value: "0",
		},
		&cli.StringFlag{
			Name:  "verified-price",
			Usage: "Set the price of the ask for verified deals (specified as FIL / GiB / Epoch) to `PRICE`",
			Value: "0",
		},
		&cli.StringFlag{
			Name:        "min-piece-size",
			Usage:       "Set minimum piece size (w/bit-padding, in bytes) in ask to `SIZE`",
			DefaultText: "256B",
			Value:       "256B",
		},
		&cli.StringFlag{
			Name:        "max-piece-size",
			Usage:       "Set maximum piece size (w/bit-padding, in bytes) in ask to `SIZE`, eg. KiB, MiB, GiB, TiB, PiB",
			DefaultText: "miner sector size",
			Value:       "0", //default to use miner's size
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("para `miner` is invalid: %w", err)
		}

		isUpdate := true
		storageAsk, err := api.MarketGetAsk(ctx, mAddr)
		if err != nil {
			if !strings.Contains(err.Error(), "record not found") {
				return err
			}
			storageAsk = &market.SignedStorageAsk{}
			isUpdate = false
		}

		pri, err := types.ParseFIL(cctx.String("price"))
		if err != nil {
			return err
		}

		vpri, err := types.ParseFIL(cctx.String("verified-price"))
		if err != nil {
			return err
		}

		dur, err := time.ParseDuration("720h0m0s")
		if err != nil {
			return fmt.Errorf("cannot parse duration: %w", err)
		}

		qty := dur.Seconds() / float64(constants.MainNetBlockDelaySecs)

		min, err := units.RAMInBytes(cctx.String("min-piece-size"))
		if err != nil {
			return fmt.Errorf("cannot parse min-piece-size to quantity of bytes: %w", err)
		}

		if min < 256 {
			return errors.New("minimum piece size (w/bit-padding) is 256B")
		}

		max, err := units.RAMInBytes(cctx.String("max-piece-size"))
		if err != nil {
			return fmt.Errorf("cannot parse max-piece-size to quantity of bytes: %w", err)
		}

		ssize, err := api.ActorSectorSize(ctx, mAddr)
		if err != nil {
			return fmt.Errorf("get miner's size %w", err)
		}

		smax := int64(ssize)

		if max == 0 {
			max = smax
		}

		if max > smax {
			return fmt.Errorf("max piece size (w/bit-padding) %s cannot exceed miner sector size %s", types.SizeStr(types.NewInt(uint64(max))), types.SizeStr(types.NewInt(uint64(smax))))
		}

		if isUpdate {
			if cctx.IsSet("price") {
				storageAsk.Ask.Price = types.BigInt(pri)
			}
			if cctx.IsSet("verified-price") {
				storageAsk.Ask.VerifiedPrice = types.BigInt(vpri)
			}
			if cctx.IsSet("min-piece-size") {
				storageAsk.Ask.MinPieceSize = abi.PaddedPieceSize(min)
			}
			if cctx.IsSet("max-piece-size") {
				storageAsk.Ask.MaxPieceSize = abi.PaddedPieceSize(max)
			}
			return api.MarketSetAsk(ctx, mAddr, storageAsk.Ask.Price, storageAsk.Ask.VerifiedPrice, abi.ChainEpoch(qty), storageAsk.Ask.MinPieceSize, storageAsk.Ask.MaxPieceSize)
		}
		return api.MarketSetAsk(ctx, mAddr, types.BigInt(pri), types.BigInt(vpri), abi.ChainEpoch(qty), abi.PaddedPieceSize(min), abi.PaddedPieceSize(max))
	},
}

var getAskCmd = &cli.Command{
	Name:      "get",
	Usage:     "Print the miner's ask",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		fnapi, closer, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("para `miner` is invalid: %w", err)
		}

		sask, err := smapi.MarketGetAsk(ctx, mAddr)
		if err != nil {
			return err
		}

		var ask *storagemarket.StorageAsk
		if sask != nil && sask.Ask != nil {
			ask = sask.Ask
		}

		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintf(w, "Price per GiB/Epoch\tVerified\tMin. Piece Size (padded)\tMax. Piece Size (padded)\tExpiry (Epoch)\tExpiry (Appx. Rem. Time)\tSeq. No.\n")
		if ask == nil {
			fmt.Fprintf(w, "<miner does not have an ask>\n")
			return w.Flush()
		}

		head, err := fnapi.ChainHead(ctx)
		if err != nil {
			return err
		}

		dlt := ask.Expiry - head.Height()
		rem := "<expired>"
		if dlt > 0 {
			rem = (time.Second * time.Duration(int64(dlt)*int64(constants.MainNetBlockDelaySecs))).String()
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%d\n", types.FIL(ask.Price), types.FIL(ask.VerifiedPrice), types.SizeStr(types.NewInt(uint64(ask.MinPieceSize))), types.SizeStr(types.NewInt(uint64(ask.MaxPieceSize))), ask.Expiry, rem, ask.SeqNo)

		return w.Flush()
	},
}
