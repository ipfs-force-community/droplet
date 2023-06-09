package cli

import (
	"fmt"
	"os"

	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"

	"github.com/ipfs-force-community/droplet/v2/cli/tablewriter"
)

var DagstoreCmd = &cli.Command{
	Name:  "dagstore",
	Usage: "Manage the dagstore on the markets subsystem",
	Subcommands: []*cli.Command{
		dagstoreListShardsCmd,
		dagstoreInitializeShardCmd,
		dagstoreRecoverShardCmd,
		dagstoreInitializeAllCmd,
		dagstoreInitializeStorageCmd,
		dagstoreGcCmd,
		dagStoreDestroyShardCmd,
	},
}

var dagstoreListShardsCmd = &cli.Command{
	Name:  "list-shards",
	Usage: "List all shards known to the dagstore, with their current status",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
		&cli.StringSliceFlag{
			Name: "filter",
			Usage: `Filter shards in specific states,
eg. ./droplet dagstore list-shards --filter=ShardStateErrored --filter=ShardStateAvailable, will ignore Errored and Available shards.
all shard states:
ShardStateAvailable
ShardStateServing
ShardStateErrored
ShardStateNew
ShardStateInitializing
ShardStateRecovering
ShardStateUnknown
`,
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		marketsApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		shards, err := marketsApi.DagstoreListShards(ctx)
		if err != nil {
			return err
		}

		if len(shards) == 0 {
			return nil
		}

		filterStates := make(map[string]struct{})
		for _, state := range cctx.StringSlice("filter") {
			filterStates[state] = struct{}{}
		}

		tw := tablewriter.New(
			tablewriter.Col("Key"),
			tablewriter.Col("State"),
			tablewriter.Col("Error"),
		)

		colors := map[string]color.Attribute{
			types.ShardStateAvailable: color.FgGreen,
			types.ShardStateServing:   color.FgBlue,
			types.ShardStateErrored:   color.FgRed,
			types.ShardStateNew:       color.FgYellow,
		}

		for _, s := range shards {
			if _, ok := filterStates[s.State]; ok {
				continue
			}
			m := map[string]interface{}{
				"Key": s.Key,
				"State": func() string {
					if c, ok := colors[s.State]; ok {
						return color.New(c).Sprint(s.State)
					}
					return s.State
				}(),
				"Error": s.Error,
			}
			tw.Write(m)
		}

		return tw.Flush(os.Stdout)
	},
}

var dagstoreInitializeShardCmd = &cli.Command{
	Name:      "initialize-shard",
	ArgsUsage: "[key]",
	Usage:     "Initialize the specified shard",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		if cctx.NArg() != 1 {
			return fmt.Errorf("must provide a single shard key")
		}

		marketsApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		return marketsApi.DagstoreInitializeShard(ctx, cctx.Args().First())
	},
}

var dagstoreRecoverShardCmd = &cli.Command{
	Name:      "recover-shard",
	ArgsUsage: "[key]",
	Usage:     "Attempt to recover a shard in errored state",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		if cctx.NArg() != 1 {
			return fmt.Errorf("must provide a single shard key")
		}

		marketsApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		return marketsApi.DagstoreRecoverShard(ctx, cctx.Args().First())
	},
}

var dagstoreInitializeAllCmd = &cli.Command{
	Name:  "initialize-all",
	Usage: "Initialize all uninitialized shards, streaming results as they're produced; only shards for unsealed pieces are initialized by default",
	Flags: []cli.Flag{
		&cli.UintFlag{
			Name:     "concurrency",
			Usage:    "maximum shards to initialize concurrently at a time; use 0 for unlimited",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "include-sealed",
			Usage: "initialize sealed pieces as well",
		},
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		concurrency := cctx.Uint("concurrency")
		sealed := cctx.Bool("sealed")

		marketsApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		params := types.DagstoreInitializeAllParams{
			MaxConcurrency: int(concurrency),
			IncludeSealed:  sealed,
		}

		ch, err := marketsApi.DagstoreInitializeAll(ctx, params)
		if err != nil {
			return err
		}

		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return nil
				}
				_, _ = fmt.Fprint(os.Stdout, color.New(color.BgHiBlack).Sprintf("(%d/%d)", evt.Current, evt.Total))
				_, _ = fmt.Fprint(os.Stdout, " ")
				if evt.Event == "start" {
					_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.Reset).Sprint("STARTING"))
				} else {
					if evt.Success {
						_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.FgGreen).Sprint("SUCCESS"))
					} else {
						_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.FgRed).Sprint("ERROR"), evt.Error)
					}
				}

			case <-ctx.Done():
				return fmt.Errorf("aborted")
			}
		}
	},
}

var dagstoreInitializeStorageCmd = &cli.Command{
	Name:  "initialize-storage",
	Usage: "Initialize all uninitialized shards in specify piece storage",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "storage",
			Usage:    "specify storage to scan for index",
			Required: true,
		},
		&cli.UintFlag{
			Name:     "concurrency",
			Usage:    "maximum shards to initialize concurrently at a time; use 0 for unlimited",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "include-sealed",
			Usage: "initialize sealed pieces as well",
		},
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		storageName := cctx.String("storage")
		concurrency := cctx.Uint("concurrency")
		sealed := cctx.Bool("sealed")

		marketsApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		params := types.DagstoreInitializeAllParams{
			MaxConcurrency: int(concurrency),
			IncludeSealed:  sealed,
		}

		ch, err := marketsApi.DagstoreInitializeStorage(ctx, storageName, params)
		if err != nil {
			return err
		}

		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return nil
				}
				_, _ = fmt.Fprint(os.Stdout, color.New(color.BgHiBlack).Sprintf("(%d/%d)", evt.Current, evt.Total))
				_, _ = fmt.Fprint(os.Stdout, " ")
				if evt.Event == "start" {
					_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.Reset).Sprint("STARTING"))
				} else {
					if evt.Success {
						_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.FgGreen).Sprint("SUCCESS"))
					} else {
						_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.FgRed).Sprint("ERROR"), evt.Error)
					}
				}

			case <-ctx.Done():
				return fmt.Errorf("aborted")
			}
		}
	},
}

var dagstoreGcCmd = &cli.Command{
	Name:  "gc",
	Usage: "Garbage collect the dagstore",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		marketsApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		collected, err := marketsApi.DagstoreGC(ctx)
		if err != nil {
			return err
		}

		if len(collected) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "no shards collected")
			return nil
		}

		for _, e := range collected {
			if e.Error == "" {
				_, _ = fmt.Fprintln(os.Stdout, e.Key, color.New(color.FgGreen).Sprint("SUCCESS"))
			} else {
				_, _ = fmt.Fprintln(os.Stdout, e.Key, color.New(color.FgRed).Sprint("ERROR"), e.Error)
			}
		}

		return nil
	},
}

var dagStoreDestroyShardCmd = &cli.Command{
	Name:  "destroy-shard",
	Usage: "Destroy shard",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "errored",
			Usage: "Destroy all errored shards",
		},
	},
	ArgsUsage: "[keys]",
	Action: func(cliCtx *cli.Context) error {
		marketsApi, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cliCtx)
		args := cliCtx.Args().Slice()
		if len(args) == 0 && !cliCtx.IsSet("errored") {
			return fmt.Errorf("must pass shard key or set --errored flag")
		}

		keys := args
		if cliCtx.Bool("errored") {
			shards, err := marketsApi.DagstoreListShards(ctx)
			if err != nil {
				return err
			}
			for _, shardInfo := range shards {
				if shardInfo.State == types.ShardStateErrored {
					keys = append(keys, shardInfo.Key)
				}
			}
		}

		fmt.Printf("Have %d shard need to destroy\n", len(keys))

		for _, key := range keys {
			if err := marketsApi.DagstoreDestroyShard(ctx, key); err != nil {
				return fmt.Errorf("destroy %s failed: %v", key, err)
			}
			fmt.Printf("destroy %s success\n", key)
		}

		return nil
	},
}
