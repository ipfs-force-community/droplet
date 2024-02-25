package cli

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/filecoin-project/go-address"
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
		importDirectDealsCmd,
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
			{"Message", deal.Message},
			{"SectorID", deal.SectorID},
			{"Offset", deal.Offset},
			{"Length", deal.Length},
			{"StartEpoch", deal.StartEpoch},
			{"EndEpoch", deal.EndEpoch},
		}

		fillSpaceAndPrint(data, len("AllocationID"))

		return nil
	},
}

var listDirectDealCmd = &cli.Command{
	Name:  "list",
	Usage: "list direct deal",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "miner",
			Usage: "provider address",
		},
		offsetFlag,
		limitFlag,
		&cli.Uint64Flag{
			Name: "state",
			Usage: `
deal states:
1  DealAllocation
2  DealSealing
3  DealActive
4  DealExpired
5  DealSlashed
6  DealError
`,
			Value: 1,
		},
		&cli.BoolFlag{
			Name:  "asc",
			Value: true,
		},
	},
	Action: func(cliCtx *cli.Context) error {
		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		params := types.DirectDealQueryParams{
			Page: types.Page{
				Offset: cliCtx.Int("offset"),
				Limit:  cliCtx.Int("limit"),
			},
			Asc: cliCtx.Bool("asc"),
		}
		if cliCtx.IsSet("miner") {
			params.Provider, err = address.NewFromString(cliCtx.String("miner"))
			if err != nil {
				return fmt.Errorf("para `miner` is invalid: %w", err)
			}
		}
		if cliCtx.IsSet("client") {
			params.Client, err = address.NewFromString(cliCtx.String("client"))
			if err != nil {
				return fmt.Errorf("para `client` is invalid: %w", err)
			}
		}

		state := types.DirectDealState(cliCtx.Uint64("state"))
		params.State = &state

		deals, err := api.ListDirectDeals(cliCtx.Context, params)
		if err != nil {
			return err
		}

		out := cliCtx.App.Writer

		sort.Slice(deals, func(i, j int) bool {
			return deals[i].CreatedAt > deals[j].CreatedAt
		})

		w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintf(w, "Creation\tID\tAllocationId\tPieceCid\tState\tClient\tProvider\tSize\tMessage\n")
		for _, deal := range deals {
			createTime := time.Unix(int64(deal.CreatedAt), 0).Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%d\t%s", createTime, deal.ID, deal.AllocationID,
				deal.PieceCID, deal.State, deal.Client, deal.Provider, deal.PieceSize, deal.Message)
		}

		return w.Flush()
	},
}

var importDirectDealCmd = &cli.Command{
	Name:      "import-deal",
	Usage:     "import direct deal",
	ArgsUsage: "<pieceCid> <file>",
	Flags: []cli.Flag{
		&cli.Uint64Flag{
			Name:     "allocation-id",
			Usage:    "",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "client",
			Usage:    "",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "skip-commp",
			Usage: "skip calculate the piece-cid, please use with caution",
		},
		&cli.BoolFlag{
			Name:  "skip-index",
			Usage: "skip generate index",
		},
		&cli.BoolFlag{
			Name:  "no-copy-car-file",
			Usage: "not copy car files to piece storage",
		},
		&cli.Uint64Flag{
			Name:  "payload-size",
			Usage: "The size of the car file",
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
		client, err := address.NewFromString(cliCtx.String("client"))
		if err != nil {
			return fmt.Errorf("para `client` is invalid: %w", err)
		}

		if cliCtx.Bool("no-copy-car-file") && cliCtx.Uint64("payload-size") == 0 {
			return fmt.Errorf("must specify payload-size when no-copy-car-file is set")
		}

		allocationID := cliCtx.Uint64("allocation-id")

		params := types.DirectDealParams{
			SkipCommP:         cliCtx.Bool("skip-commp"),
			SkipGenerateIndex: cliCtx.Bool("skip-generate-index"),
			NoCopyCarFile:     cliCtx.Bool("no-copy-car-file"),
			DealParams: []types.DirectDealParam{
				{
					DealUUID:     uuid.New(),
					AllocationID: allocationID,
					PayloadSize:  cliCtx.Uint64("payload-size"),
					Client:       client,
					PieceCID:     pieceCid,
					FilePath:     filepath,
				},
			},
		}

		return api.ImportDirectDeal(cliCtx.Context, &params)
	},
}

var importDirectDealsCmd = &cli.Command{
	Name:  "import-deals",
	Usage: "import direct deal",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name: "allocation-info",
			Usage: "Allocation id and piece cid and client, separated by comma. " +
				"e.g. --allocation-id-piece 1:QmTzXp8PqXgX8i9qUQn4UzJtC7aCqkLp2qJn7Rq2JyH1D:t01001 --allocation-id-piece 2:QmTzXp8PqXgX8i9qUQn4UzJtC7aCqkLp2qJn7Rq2JyH1D:t01001",
		},
		&cli.StringFlag{
			Name: "allocation-file",
		},
		&cli.StringFlag{
			Name:  "car-dir",
			Usage: "Car file directory",
		},
		&cli.BoolFlag{
			Name:  "skip-commp",
			Usage: "skip calculate the piece-cid, please use with caution",
		},
		&cli.BoolFlag{
			Name:  "skip-index",
			Usage: "skip generate index",
		},
		&cli.BoolFlag{
			Name:  "no-copy-car-file",
			Usage: "not copy car files to piece storage",
		},
	},
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.IsSet("allocation-id-piece") == cliCtx.IsSet("allocation-file") {
			return fmt.Errorf("must specify one of allocation-id or allocation-file")
		}

		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		carDir := cliCtx.String("car-dir")

		var directDealParams []types.DirectDealParam
		if cliCtx.IsSet("allocation-info") {
			for _, ai := range cliCtx.StringSlice("allocation-info") {
				parts := strings.Split(ai, ":")
				if len(parts) != 3 {
					return fmt.Errorf("invalid allocation-id and piece cid pair: %s", ai)
				}
				allocationID, err := strconv.ParseUint(parts[0], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid allocation-id: %w", err)
				}
				pieceCid, err := cid.Decode(parts[1])
				if err != nil {
					return fmt.Errorf("invalid piece cid: %w", err)
				}
				client, err := address.NewFromString(parts[2])
				if err != nil {
					return fmt.Errorf("invalid client: %w", err)
				}
				param := types.DirectDealParam{
					DealUUID:     uuid.New(),
					AllocationID: allocationID,
					PieceCID:     pieceCid,
					Client:       client,
				}

				if len(carDir) == 0 {
					return fmt.Errorf("must specify car-dir")
				}
				param.FilePath = filepath.Join(carDir, pieceCid.String())
				directDealParams = append(directDealParams, param)
			}
		}
		if cliCtx.IsSet("allocation-file") {
			allocations, err := loadAllocations(cliCtx.String("allocation-file"))
			if err != nil {
				return fmt.Errorf("failed to load allocations: %w", err)
			}
			for _, a := range allocations {
				param := types.DirectDealParam{
					DealUUID:     uuid.New(),
					AllocationID: a.AllocationID,
					Client:       a.Client,
					PieceCID:     a.PieceCID,
					PayloadSize:  a.PayloadSize,
				}
				if param.PayloadSize == 0 && len(carDir) == 0 {
					return fmt.Errorf("must specify car-dir")
				}
				if len(carDir) != 0 {
					param.FilePath = filepath.Join(carDir, a.PieceCID.String())
				}
				directDealParams = append(directDealParams, param)
			}
		}

		params := types.DirectDealParams{
			SkipCommP:         cliCtx.Bool("skip-commp"),
			SkipGenerateIndex: cliCtx.Bool("skip-generate-index"),
			NoCopyCarFile:     cliCtx.Bool("no-copy-car-file"),
			DealParams:        directDealParams,
		}

		if err := api.ImportDirectDeal(cliCtx.Context, &params); err != nil {
			return err
		}
		fmt.Println("import deal success")

		return nil
	},
}

type allocationWithPiece struct {
	AllocationID uint64
	Client       address.Address
	PieceCID     cid.Cid
	PayloadSize  uint64
}

func loadAllocations(path string) ([]*allocationWithPiece, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return nil, err
	}

	var allocations []*allocationWithPiece
	for _, record := range records {
		if len(record) < 3 {
			return nil, fmt.Errorf("invalid record: %s", record)
		}
		allocationID, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			return nil, err
		}
		pieceCID, err := cid.Decode(record[1])
		if err != nil {
			return nil, err
		}
		client, err := address.NewFromString(record[2])
		if err != nil {
			return nil, err
		}
		var payloadSize uint64
		if len(record) == 4 {
			payloadSize, err = strconv.ParseUint(record[3], 10, 64)
			if err != nil {
				return nil, err
			}
		}

		allocations = append(allocations, &allocationWithPiece{AllocationID: allocationID, Client: client,
			PieceCID: pieceCID, PayloadSize: payloadSize})
	}

	return allocations, nil
}
