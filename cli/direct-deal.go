package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"
)

var directDealCmds = &cli.Command{
	Name:  "direct-deal",
	Usage: "the tool for direct deal",
	Subcommands: []*cli.Command{
		getDirectDeal,
		listDirectDealCmd,
		importDirectDealCmd,
	},
}

var getDirectDeal = &cli.Command{
	Name:      "get",
	Usage:     "Print a direct deal by id",
	ArgsUsage: "<id>",
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.Args().Len() < 1 {
			return fmt.Errorf("must pass id")
		}

		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		id, err := uuid.Parse(cliCtx.Args().First())
		if err != nil {
			return err
		}

		deal, err := api.GetDirectDeal(cliCtx.Context, id)
		if err != nil {
			return err
		}

		data := []kv{
			{"Creation", time.Unix(int64(deal.CreatedAt), 0).Format(time.RFC3339)},
			{"PieceCID", deal.PieceCID},
			{"PieceSize", deal.PieceSize},
			{"Client", deal.Client},
			{"Provider", deal.Provider},
			{"AllocationID", deal.AllocationID},
			{"ClaimID", deal.ClaimID},
			{"State", deal.State.String()},
			{"PieceStatus", deal.PieceStatus},
			{"Message", deal.Message},
			{"SectorID", deal.SectorID},
			{"Offset", deal.Offset},
			{"Length", deal.Length},
			{"StartEpoch", deal.StartEpoch},
			{"EndEpoch", deal.EndEpoch},
		}

		fillSpaceAndPrint(data, len(""))

		return nil
	},
}

var listDirectDealCmd = &cli.Command{
	Name:  "list",
	Usage: "list direct deal",
	Flags: []cli.Flag{},
	Action: func(cliCtx *cli.Context) error {
		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		deals, err := api.ListDirectDeals(cliCtx.Context)
		if err != nil {
			return err
		}

		out := cliCtx.App.Writer

		sort.Slice(deals, func(i, j int) bool {
			return deals[i].CreatedAt > deals[j].CreatedAt
		})

		w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintf(w, "Creation\tID\tAllocationId\tPieceCid\tState\tPieceState\tClient\tProvider\tSize\tMessage\n")
		for _, deal := range deals {
			createTime := time.Unix(int64(deal.CreatedAt), 0).Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%d\t%s", createTime, deal.ID, deal.AllocationID,
				deal.PieceCID, deal.State, deal.PieceStatus, deal.Client, deal.Provider, deal.PieceSize, deal.Message)
		}

		return nil
	},
}

var importDirectDealCmd = &cli.Command{
	Name:      "import-deal",
	Usage:     "import direct deal",
	ArgsUsage: "<pieceCid> <file>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "client",
			Usage:    "",
			Required: true,
		},
		&cli.Uint64Flag{
			Name:     "allocation-id",
			Usage:    "",
			Required: true,
		},
		&cli.IntFlag{
			Name:  "start-epoch",
			Usage: "start epoch by when the deal should be proved by provider on-chain",
			Value: 35000, // default is 35000, handy for tests with 2k/devnet build
		},
		&cli.IntFlag{
			Name:  "duration",
			Usage: "duration of the deal in epochs",
			Value: 518400, // default is 2880 * 180 == 180 days
		},
		&cli.BoolFlag{
			Name:  "skip-commp",
			Usage: "skip calculate the piece-cid, please use with caution",
		},
		&cli.BoolFlag{
			Name:  "no-copy-car-file",
			Usage: "not copy car files to piece storage and skip calculate the piece-cid",
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
		},
	},
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.Args().Len() < 2 {
			return fmt.Errorf("must specify piececid and file path")
		}

		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		pieceCidStr := cliCtx.Args().Get(0)
		path := cliCtx.Args().Get(1)

		fullPath, err := homedir.Expand(path)
		if err != nil {
			return fmt.Errorf("expanding file path: %w", err)
		}

		filepath, err := filepath.Abs(fullPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for file: %w", err)
		}

		_, err = os.Stat(filepath)
		if err != nil {
			return fmt.Errorf("opening file %s: %w", filepath, err)
		}

		pieceCid, err := cid.Decode(pieceCidStr)
		if err != nil {
			return fmt.Errorf("could not parse piece cid: %w", err)
		}

		clientAddr, err := address.NewFromString(cliCtx.String("client"))
		if err != nil {
			return fmt.Errorf("failed to parse client param: %w", err)
		}

		allocationID := cliCtx.Uint64("allocation-id")

		startEpoch := abi.ChainEpoch(cliCtx.Int("start-epoch"))
		endEpoch := startEpoch + abi.ChainEpoch(cliCtx.Int("duration"))

		params := types.DirectDealParams{
			PieceCID:      pieceCid,
			FilePath:      filepath,
			SkipCommP:     cliCtx.Bool("skip-commp"),
			NoCopyCarFile: cliCtx.Bool("no-copy-car-file"),
			AllocationID:  shared.AllocationId(allocationID),
			Client:        clientAddr,
			StartEpoch:    startEpoch,
			EndEpoch:      endEpoch,
		}

		return api.ImportDirectDeal(cliCtx.Context, &params)
	},
}
