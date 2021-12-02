package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	tm "github.com/buger/goterm"
	"github.com/docker/go-units"
	"github.com/fatih/color"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/pkg/types"

	cli2 "github.com/filecoin-project/venus-market/cli"
	"github.com/filecoin-project/venus-market/cli/tablewriter"
	"github.com/filecoin-project/venus-market/client"
)

var retrievalCmd = &cli.Command{
	Name:  "retrieval",
	Usage: "manage retrieval deals",
	Subcommands: []*cli.Command{
		retrievalFindCmd,
		retrievalRetrieveCmd,
		retrievalCancelCmd,
		retrievalListCmd,
	},
}

var retrievalFindCmd = &cli.Command{
	Name:      "find",
	Usage:     "Find data in the network",
	ArgsUsage: "[dataCid]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "pieceCid",
			Usage: "require data to be retrieved from a specific Piece CID",
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			fmt.Println("Usage: find [CID]")
			return nil
		}

		file, err := cid.Parse(cctx.Args().First())
		if err != nil {
			return err
		}

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		// Check if we already have this data locally

		has, err := api.ClientHasLocal(ctx, file)
		if err != nil {
			return err
		}

		if has {
			fmt.Println("LOCAL")
		}

		var pieceCid *cid.Cid
		if cctx.String("pieceCid") != "" {
			parsed, err := cid.Parse(cctx.String("pieceCid"))
			if err != nil {
				return err
			}
			pieceCid = &parsed
		}

		offers, err := api.ClientFindData(ctx, file, pieceCid)
		if err != nil {
			return err
		}

		for _, offer := range offers {
			if offer.Err != "" {
				fmt.Printf("ERR %s@%s: %s\n", offer.Miner, offer.MinerPeer.ID, offer.Err)
				continue
			}
			fmt.Printf("RETRIEVAL %s@%s-%s-%s\n", offer.Miner, offer.MinerPeer.ID, types.FIL(offer.MinPrice), types.SizeStr(types.NewInt(offer.Size)))
		}

		return nil
	},
}

const DefaultMaxRetrievePrice = "0.01"

var retrievalRetrieveCmd = &cli.Command{
	Name:      "retrieve",
	Usage:     "Retrieve data from network",
	ArgsUsage: "[dataCid outputPath]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "address to send transactions from",
		},
		&cli.BoolFlag{
			Name:  "car",
			Usage: "export to a car file instead of a regular file",
		},
		&cli.StringFlag{
			Name:  "miner",
			Usage: "miner address for retrieval, if not present it'll use local discovery",
		},
		&cli.StringFlag{
			Name:  "maxPrice",
			Usage: fmt.Sprintf("maximum price the client is willing to consider (default: %s FIL)", DefaultMaxRetrievePrice),
		},
		&cli.StringFlag{
			Name:  "pieceCid",
			Usage: "require data to be retrieved from a specific Piece CID",
		},
		&cli.BoolFlag{
			Name: "allow-local",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.ShowHelp(cctx, fmt.Errorf("incorrect number of arguments"))
		}
		mapi, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)
		afmt := cli2.NewAppFmt(cctx.App)

		var payer address.Address
		if cctx.String("from") != "" {
			payer, err = address.NewFromString(cctx.String("from"))
		} else {
			payer, err = mapi.DefaultAddress(ctx)
		}
		if err != nil {
			return err
		}

		file, err := cid.Parse(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		var pieceCid *cid.Cid
		if cctx.String("pieceCid") != "" {
			parsed, err := cid.Parse(cctx.String("pieceCid"))
			if err != nil {
				return err
			}
			pieceCid = &parsed
		}

		var order *client.RetrievalOrder
		if cctx.Bool("allow-local") {
			imports, err := mapi.ClientListImports(ctx)
			if err != nil {
				return err
			}

			for _, i := range imports {
				if i.Root != nil && i.Root.Equals(file) {
					order = &client.RetrievalOrder{
						Root:         file,
						FromLocalCAR: i.CARPath,

						Total:       big.Zero(),
						UnsealPrice: big.Zero(),
					}
					break
				}
			}
		}

		if order == nil {
			var offer client.QueryOffer
			minerStrAddr := cctx.String("miner")
			if minerStrAddr == "" { // Local discovery
				offers, err := mapi.ClientFindData(ctx, file, pieceCid)

				var cleaned []client.QueryOffer
				// filter out offers that errored
				for _, o := range offers {
					if o.Err == "" {
						cleaned = append(cleaned, o)
					}
				}

				offers = cleaned

				// sort by price low to high
				sort.Slice(offers, func(i, j int) bool {
					return offers[i].MinPrice.LessThan(offers[j].MinPrice)
				})
				if err != nil {
					return err
				}

				// TODO: parse offer strings from `client find`, make this smarter
				if len(offers) < 1 {
					fmt.Println("Failed to find file")
					return nil
				}
				offer = offers[0]
			} else { // Directed retrieval
				minerAddr, err := address.NewFromString(minerStrAddr)
				if err != nil {
					return err
				}
				offer, err = mapi.ClientMinerQueryOffer(ctx, minerAddr, file, pieceCid)
				if err != nil {
					return err
				}
			}
			if offer.Err != "" {
				return fmt.Errorf("The received offer errored: %s", offer.Err)
			}

			maxPrice := types.MustParseFIL(DefaultMaxRetrievePrice)

			if cctx.String("maxPrice") != "" {
				maxPrice, err = types.ParseFIL(cctx.String("maxPrice"))
				if err != nil {
					return xerrors.Errorf("parsing maxPrice: %w", err)
				}
			}

			if offer.MinPrice.GreaterThan(big.Int(maxPrice)) {
				return xerrors.Errorf("failed to find offer satisfying maxPrice: %s", maxPrice)
			}

			o := offer.Order(payer)
			order = &o
		}
		ref := &client.FileRef{
			Path:  cctx.Args().Get(1),
			IsCAR: cctx.Bool("car"),
		}

		fmt.Println("Size:", order.Size)
		fmt.Println("Unseal Price:", order.UnsealPrice)
		fmt.Println("Total Fee:", order.Total)

		data, _ := json.MarshalIndent(order, " ", "\t")
		fmt.Println(string(data))
		updates, err := mapi.ClientRetrieveWithEvents(ctx, *order, ref)
		if err != nil {
			return xerrors.Errorf("error setting up retrieval: %w", err)
		}

		var prevStatus retrievalmarket.DealStatus

		for {
			select {
			case evt, ok := <-updates:
				if ok {
					afmt.Printf("> Recv: %s, Paid %s, %s (%s)\n",
						types.SizeStr(types.NewInt(evt.BytesReceived)),
						types.FIL(evt.FundsSpent),
						retrievalmarket.ClientEvents[evt.Event],
						retrievalmarket.DealStatuses[evt.Status],
					)
					prevStatus = evt.Status
				}

				if evt.Err != "" {
					return xerrors.Errorf("retrieval failed: %s", evt.Err)
				}

				if !ok {
					if prevStatus == retrievalmarket.DealStatusCompleted {
						afmt.Println("Success")
					} else {
						afmt.Printf("saw final deal state %s instead of expected success state DealStatusCompleted\n",
							retrievalmarket.DealStatuses[prevStatus])
					}
					return nil
				}

			case <-ctx.Done():
				return xerrors.Errorf("retrieval timed out")
			}
		}
	},
}

func retrievalStatusString(status retrievalmarket.DealStatus) string {
	s := retrievalmarket.DealStatuses[status]

	switch {
	case isTerminalError(status):
		return color.RedString(s)
	case retrievalmarket.IsTerminalSuccess(status):
		return color.GreenString(s)
	default:
		return s
	}
}
func isTerminalError(status retrievalmarket.DealStatus) bool {
	// should patch this in go-fil-markets but to solve the problem immediate and not have buggy output
	return retrievalmarket.IsTerminalError(status) || status == retrievalmarket.DealStatusErrored || status == retrievalmarket.DealStatusCancelled
}
func toRetrievalOutput(d client.RetrievalInfo, verbose bool) map[string]interface{} {

	payloadCID := d.PayloadCID.String()
	provider := d.Provider.String()
	if !verbose {
		payloadCID = ellipsis(payloadCID, 8)
		provider = ellipsis(provider, 8)
	}

	retrievalOutput := map[string]interface{}{
		"PayloadCID":   payloadCID,
		"DealId":       d.ID,
		"Provider":     provider,
		"Status":       retrievalStatusString(d.Status),
		"PricePerByte": types.FIL(d.PricePerByte),
		"Received":     units.BytesSize(float64(d.BytesReceived)),
		"TotalPaid":    types.FIL(d.TotalPaid),
		"Message":      d.Message,
	}

	if verbose {
		transferChannelID := ""
		if d.TransferChannelID != nil {
			transferChannelID = d.TransferChannelID.String()
		}
		transferStatus := ""
		if d.DataTransfer != nil {
			transferStatus = datatransfer.Statuses[d.DataTransfer.Status]
		}
		pieceCID := ""
		if d.PieceCID != nil {
			pieceCID = d.PieceCID.String()
		}

		retrievalOutput["PieceCID"] = pieceCID
		retrievalOutput["UnsealPrice"] = types.FIL(d.UnsealPrice)
		retrievalOutput["BytesPaidFor"] = units.BytesSize(float64(d.BytesPaidFor))
		retrievalOutput["TransferChannelID"] = transferChannelID
		retrievalOutput["TransferStatus"] = transferStatus
	}
	return retrievalOutput
}
func outputRetrievalDeals(ctx context.Context, out io.Writer, localDeals []client.RetrievalInfo, verbose bool, showFailed bool, completed bool) error {
	var deals []client.RetrievalInfo
	for _, deal := range localDeals {
		if !showFailed && isTerminalError(deal.Status) {
			continue
		}
		if !completed && retrievalmarket.IsTerminalSuccess(deal.Status) {
			continue
		}
		deals = append(deals, deal)
	}

	tableColumns := []tablewriter.Column{
		tablewriter.Col("PayloadCID"),
		tablewriter.Col("DealId"),
		tablewriter.Col("Provider"),
		tablewriter.Col("Status"),
		tablewriter.Col("PricePerByte"),
		tablewriter.Col("Received"),
		tablewriter.Col("TotalPaid"),
	}

	if verbose {
		tableColumns = append(tableColumns,
			tablewriter.Col("PieceCID"),
			tablewriter.Col("UnsealPrice"),
			tablewriter.Col("BytesPaidFor"),
			tablewriter.Col("TransferChannelID"),
			tablewriter.Col("TransferStatus"),
		)
	}
	tableColumns = append(tableColumns, tablewriter.NewLineCol("Message"))

	w := tablewriter.New(tableColumns...)

	for _, d := range deals {
		w.Write(toRetrievalOutput(d, verbose))
	}

	return w.Flush(out)
}

var retrievalListCmd = &cli.Command{
	Name:  "list-retrievals",
	Usage: "List retrieval market deals",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "print verbose deal details",
		},
		&cli.BoolFlag{
			Name:        "color",
			Usage:       "use color in display output",
			DefaultText: "depends on output being a TTY",
		},
		&cli.BoolFlag{
			Name:  "show-failed",
			Usage: "show failed/failing deals",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "completed",
			Usage: "show completed retrievals",
		},
		&cli.BoolFlag{
			Name:  "watch",
			Usage: "watch deal updates in real-time, rather than a one time list",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		verbose := cctx.Bool("verbose")
		watch := cctx.Bool("watch")
		showFailed := cctx.Bool("show-failed")
		completed := cctx.Bool("completed")

		localDeals, err := api.ClientListRetrievals(ctx)
		if err != nil {
			return err
		}

		if watch {
			updates, err := api.ClientGetRetrievalUpdates(ctx)
			if err != nil {
				return err
			}

			for {
				tm.Clear()
				tm.MoveCursor(1, 1)

				err = outputRetrievalDeals(ctx, tm.Screen, localDeals, verbose, showFailed, completed)
				if err != nil {
					return err
				}

				tm.Flush()

				select {
				case <-ctx.Done():
					return nil
				case updated := <-updates:
					var found bool
					for i, existing := range localDeals {
						if existing.ID == updated.ID {
							localDeals[i] = updated
							found = true
							break
						}
					}
					if !found {
						localDeals = append(localDeals, updated)
					}
				}
			}
		}

		return outputRetrievalDeals(ctx, cctx.App.Writer, localDeals, verbose, showFailed, completed)
	},
}

var retrievalCancelCmd = &cli.Command{
	Name:  "cancel-retrieval",
	Usage: "Cancel a retrieval deal by deal ID; this also cancels the associated transfer",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:     "deal-id",
			Usage:    "specify retrieval deal by deal ID",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		id := cctx.Int64("deal-id")
		if id < 0 {
			return errors.New("deal id cannot be negative")
		}

		return api.ClientCancelRetrievalDeal(ctx, retrievalmarket.DealID(id))
	},
}
