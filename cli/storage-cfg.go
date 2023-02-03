package cli

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/urfave/cli/v2"
)

var storageCfgCmds = &cli.Command{
	Name:  "cfg",
	Usage: "Configure storage config",
	Subcommands: []*cli.Command{
		storageDealSelectionCmds,
		blocksListCmds,
		expectedSealDurationCmds,
		maxDealStartDelayCmds,
		dealsPublishMsgPeriodCmds,
	},
}

var storageDealSelectionCmds = &cli.Command{
	Name:  "selection",
	Usage: "Configure acceptance criteria for storage deal proposals",
	Subcommands: []*cli.Command{
		storageDealSelectionShowCmd,
		storageDealSelectionResetCmd,
		storageDealSelectionRejectCmd,
	},
}

var storageDealSelectionShowCmd = &cli.Command{
	Name:      "list",
	Usage:     "List storage deal proposal selection criteria",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		onlineOk, err := smapi.DealsConsiderOnlineStorageDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		offlineOk, err := smapi.DealsConsiderOfflineStorageDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		verifiedOk, err := smapi.DealsConsiderVerifiedStorageDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		unverifiedOk, err := smapi.DealsConsiderUnverifiedStorageDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		fmt.Printf("considering online storage deals: %t\n", onlineOk)
		fmt.Printf("considering offline storage deals: %t\n", offlineOk)
		fmt.Printf("considering verified storage deals: %t\n", verifiedOk)
		fmt.Printf("considering unverified storage deals: %t\n", unverifiedOk)

		return nil
	},
}

var storageDealSelectionResetCmd = &cli.Command{
	Name:      "reset",
	Usage:     "Reset storage deal proposal selection criteria to default values",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must set miner address argument")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		err = smapi.DealsSetConsiderOnlineStorageDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		err = smapi.DealsSetConsiderOfflineStorageDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		err = smapi.DealsSetConsiderVerifiedStorageDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		err = smapi.DealsSetConsiderUnverifiedStorageDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		return nil
	},
}

var storageDealSelectionRejectCmd = &cli.Command{
	Name:      "reject",
	Usage:     "Configure criteria which necessitate automatic rejection",
	ArgsUsage: "<miner address>",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "online",
		},
		&cli.BoolFlag{
			Name: "offline",
		},
		&cli.BoolFlag{
			Name: "verified",
		},
		&cli.BoolFlag{
			Name: "unverified",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.Bool("online") {
			err = smapi.DealsSetConsiderOnlineStorageDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		if cctx.Bool("offline") {
			err = smapi.DealsSetConsiderOfflineStorageDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		if cctx.Bool("verified") {
			err = smapi.DealsSetConsiderVerifiedStorageDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		if cctx.Bool("unverified") {
			err = smapi.DealsSetConsiderUnverifiedStorageDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var blocksListCmds = &cli.Command{
	Name:  "block-list",
	Usage: "Configure miner's CID block list",
	Subcommands: []*cli.Command{
		getBlocklistCmd,
		setBlocklistCmd,
		resetBlocklistCmd,
	},
}

var getBlocklistCmd = &cli.Command{
	Name:      "get",
	Usage:     "List the contents of the miner's piece CID blocklist",
	ArgsUsage: "<miner address>",
	Flags: []cli.Flag{
		&CidBaseFlag,
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		blocklist, err := api.DealsPieceCidBlocklist(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		encoder, err := GetCidEncoder(cctx)
		if err != nil {
			return err
		}

		for idx := range blocklist {
			fmt.Println(encoder.Encode(blocklist[idx]))
		}

		return nil
	},
}

var setBlocklistCmd = &cli.Command{
	Name:      "set",
	Usage:     "Set the miner's list of blocklisted piece CIDs",
	ArgsUsage: "[<miner address> <path-of-file-containing-newline-delimited-piece-CIDs> (optional, will read from stdin if omitted)]",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() == 0 {
			return errors.New("need at least one argument for miner address argument")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		scanner := bufio.NewScanner(os.Stdin)
		if cctx.NArg() == 2 && cctx.Args().Get(1) != "-" {
			absPath, err := filepath.Abs(cctx.Args().Get(1))
			if err != nil {
				return err
			}

			file, err := os.Open(absPath)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close() //nolint:errcheck

			scanner = bufio.NewScanner(file)
		}

		var blocklist []cid.Cid
		for scanner.Scan() {
			decoded, err := cid.Decode(scanner.Text())
			if err != nil {
				return err
			}

			blocklist = append(blocklist, decoded)
		}

		err = scanner.Err()
		if err != nil {
			return err
		}

		return api.DealsSetPieceCidBlocklist(DaemonContext(cctx), mAddr, blocklist)
	},
}

var resetBlocklistCmd = &cli.Command{
	Name:      "reset",
	Usage:     "Remove all entries from the miner's piece CID blocklist",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		return api.DealsSetPieceCidBlocklist(DaemonContext(cctx), mAddr, []cid.Cid{})
	},
}

var expectedSealDurationCmds = &cli.Command{
	Name:  "seal-duration",
	Usage: "Configure the expected time, that you expect sealing sectors to take. Deals that start before this duration will be rejected.",
	Subcommands: []*cli.Command{
		expectedSealDurationGetCmd,
		expectedSealDurationSetCmd,
	},
}

var expectedSealDurationGetCmd = &cli.Command{
	Name:      "get",
	Usage:     "set miner's expected seal duration",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)
		t, err := marketApi.SectorGetExpectedSealDuration(ctx, mAddr)
		if err != nil {
			return err
		}
		fmt.Println("seal-duration: ", t.String())
		return nil
	},
}

var expectedSealDurationSetCmd = &cli.Command{
	Name:      "set",
	Usage:     "eg. '1m','30s',...",
	ArgsUsage: "<miner address> <duration>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return errors.New("must miner address and time duration arguments")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)
		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must pass duration")
		}

		d, err := time.ParseDuration(cctx.Args().Get(1))
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}

		return marketApi.SectorSetExpectedSealDuration(ctx, mAddr, d)
	},
}

var maxDealStartDelayCmds = &cli.Command{
	Name:  "max-start-delay",
	Usage: "Configure the maximum amount of time proposed deal StartEpoch can be in future.",
	Subcommands: []*cli.Command{
		maxDealStartDelayGetCmd,
		maxDealStartDelaySetCmd,
	},
}

var maxDealStartDelayGetCmd = &cli.Command{
	Name:      "get",
	Usage:     "config miner's max deal start delay",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)
		t, err := marketApi.DealsMaxStartDelay(ctx, mAddr)
		if err != nil {
			return err
		}
		fmt.Println("max start delay: ", t.String())
		return nil
	},
}

var maxDealStartDelaySetCmd = &cli.Command{
	Name:      "set",
	Usage:     "eg. '1m','30s',...",
	ArgsUsage: "<miner address> <minutes>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return errors.New("must miner address and time duration arguments")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)
		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must pass duration")
		}

		delay, err := time.ParseDuration(cctx.Args().Get(1))
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}

		return marketApi.DealsSetMaxStartDelay(ctx, mAddr, delay)
	},
}

var dealsPublishMsgPeriodCmds = &cli.Command{
	Name:  "publish-period",
	Usage: "Configure the the amount of time to wait for more deals to be ready to publish before publishing them all as a batch.",
	Subcommands: []*cli.Command{
		dealsPublishMsgPeriodGetCmd,
		dealsPublishMsgPeriodSetCmd,
	},
}

var dealsPublishMsgPeriodGetCmd = &cli.Command{
	Name:      "get",
	Usage:     "config miner's period of publishing message",
	ArgsUsage: "<miner address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)
		t, err := marketApi.DealsPublishMsgPeriod(ctx, mAddr)
		if err != nil {
			return err
		}
		fmt.Println("publish msg period: ", t.String())
		return nil
	},
}

var dealsPublishMsgPeriodSetCmd = &cli.Command{
	Name:      "set",
	Usage:     "eg. '1m','30s',...",
	ArgsUsage: "<miner address> <duration>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return errors.New("must miner address and time duration arguments")
		}
		mAddr, err := shouldAddress(cctx.Args().Get(0), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)
		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must pass duration")
		}

		period, err := time.ParseDuration(cctx.Args().Get(1))
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}
		return marketApi.DealsSetPublishMsgPeriod(ctx, mAddr, period)
	},
}
