package cli

import (
	"errors"
	"fmt"

	"github.com/urfave/cli/v2"
)

// If more configurations appear in the future, they need to be changed to  retrieval-cfg.
var retrievalDealSelectionCmds = &cli.Command{
	Name:  "selection",
	Usage: "Configure acceptance criteria for retrieval deal proposals",
	Subcommands: []*cli.Command{
		retrievalDealSelectionShowCmd,
		retrievalDealSelectionResetCmd,
		retrievalDealSelectionRejectCmd,
	},
}

var retrievalDealSelectionShowCmd = &cli.Command{
	Name:      "list",
	ArgsUsage: "<miner address>",
	Usage:     "List retrieval deal proposal selection criteria",
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

		onlineOk, err := smapi.DealsConsiderOnlineRetrievalDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		offlineOk, err := smapi.DealsConsiderOfflineRetrievalDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		fmt.Printf("considering online retrieval deals: %t\n", onlineOk)
		fmt.Printf("considering offline retrieval deals: %t\n", offlineOk)

		return nil
	},
}

var retrievalDealSelectionResetCmd = &cli.Command{
	Name:      "reset",
	Usage:     "Reset retrieval deal proposal selection criteria to default values",
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

		err = smapi.DealsSetConsiderOnlineRetrievalDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		err = smapi.DealsSetConsiderOfflineRetrievalDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		return nil
	},
}

var retrievalDealSelectionRejectCmd = &cli.Command{
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
			err = smapi.DealsSetConsiderOnlineRetrievalDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		if cctx.Bool("offline") {
			err = smapi.DealsSetConsiderOfflineRetrievalDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		return nil
	},
}
