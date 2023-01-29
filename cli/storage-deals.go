package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	tm "github.com/buger/goterm"
	"github.com/docker/go-units"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/filecoin-project/venus-market/v2/storageprovider"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
)

var StorageCmds = &cli.Command{
	Name:  "storage",
	Usage: "Manage storage deals and related configuration",
	Subcommands: []*cli.Command{
		storageDealsCmds,
		storageAsksCmds,
		storageCfgCmds,
	},
}

var storageDealsCmds = &cli.Command{
	Name:  "deal",
	Usage: "Manage storage deals and related configuration",
	Subcommands: []*cli.Command{
		dealsImportDataCmd,
		importOfflineDealCmd,
		dealsListCmd,
		updateStorageDealStateCmd,
		dealsPendingPublish,
	},
}

var dealsImportDataCmd = &cli.Command{
	Name:      "import-data",
	Usage:     "Manually import data for a deal",
	ArgsUsage: "<proposal CID> <file>",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := DaemonContext(cctx)

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("must specify proposal CID and file path")
		}

		propCid, err := cid.Decode(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		fpath := cctx.Args().Get(1)

		return api.DealsImportData(ctx, propCid, fpath)
	},
}

var importOfflineDealCmd = &cli.Command{
	Name:      "import-offlinedeal",
	Usage:     "Manually import offline deal",
	ArgsUsage: "<deal_file_json>",
	Flags: []cli.Flag{
		// verbose
		&cli.BoolFlag{
			Name:  "verbose",
			Usage: "Print verbose output",
			Aliases: []string{
				"v",
			},
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := DaemonContext(cctx)

		if cctx.Args().Len() < 1 {
			return fmt.Errorf("must specify the path of json file which records the deal")
		}

		fpath := cctx.Args().Get(0)

		dealbyte, err := ioutil.ReadFile(fpath)
		if err != nil {
			return fmt.Errorf("read deal file(%s) fail %w", fpath, err)
		}

		data := []market.MinerDeal{}
		err = json.Unmarshal(dealbyte, &data)
		if err != nil {
			return fmt.Errorf("parse deal file(%s) fail %w", fpath, err)
		}

		totalCount := len(data)
		importedCount := 0

		// if verbose, print the deal info

		for i := 0; i < totalCount; i++ {
			err := api.OfflineDealImport(ctx, data[i])
			if err != nil {
				if cctx.Bool("verbose") {
					fmt.Printf("( %d / %d ) %s : fail : %v\n", i+1, totalCount, data[i].ProposalCid, err)
				}
			} else {
				importedCount++
				if cctx.Bool("verbose") {
					fmt.Printf("( %d / %d ) %s : success\n", i+1, totalCount, data[i].ProposalCid)
				}
			}
		}

		fmt.Printf("import %d deals, %d deal success , %d deal fail .\n", totalCount, importedCount, totalCount-importedCount)

		return nil
	},
}

var dealsListCmd = &cli.Command{
	Name:  "list",
	Usage: "List all deals for this miner",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
		},
		&cli.BoolFlag{
			Name:  "watch",
			Usage: "watch deal updates in real-time, rather than a one time list",
		},
		&cli.StringFlag{
			Name: "miner",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		maddr := address.Undef
		if cctx.IsSet("miner") {
			maddr, err = address.NewFromString(cctx.String("miner"))
			if err != nil {
				return fmt.Errorf("para `miner` is invalid: %w", err)
			}
		}

		ctx := DaemonContext(cctx)
		deals, err := api.MarketListIncompleteDeals(ctx, maddr)
		if err != nil {
			return err
		}

		verbose := cctx.Bool("verbose")
		watch := cctx.Bool("watch")

		if watch {
			updates, err := api.MarketGetDealUpdates(ctx)
			if err != nil {
				return err
			}

			for {
				tm.Clear()
				tm.MoveCursor(1, 1)

				err = outputStorageDeals(tm.Output, deals, verbose)
				if err != nil {
					return err
				}

				tm.Flush()

				select {
				case <-ctx.Done():
					return nil
				case updated := <-updates:
					var found bool
					for i, existing := range deals {
						if existing.ProposalCid.Equals(updated.ProposalCid) {
							deals[i] = updated
							found = true
							break
						}
					}
					if !found {
						deals = append(deals, updated)
					}
				}
			}
		}

		return outputStorageDeals(os.Stdout, deals, verbose)
	},
}

var dealStateUsage = func() string {
	const c, spliter = 5, " | "
	size := len(storageprovider.StringToStorageState)
	states := make([]string, 0, size+size/c)
	idx := 0
	for s := range storageprovider.StringToStorageState {
		states = append(states, s)
		idx++
		states = append(states, spliter)
		if idx%c == 0 {
			states = append(states, "\n\t")
			continue
		}
	}

	usage := strings.Join(states, "")
	{
		size := len(usage)
		if size > 3 && usage[size-3:] == spliter {
			usage = usage[:size-3]
		}
	}
	return usage + ", set to 'StorageDealUnknown' means no change"
}

var updateStorageDealStateCmd = &cli.Command{
	Name:  "update",
	Usage: "update deal status",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "proposalcid",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
			Value: false,
		},
		&cli.StringFlag{
			Name:  "piece-state",
			Usage: "Undefine | Assigned | Packing | Proving, empty means no change",
		},
		&cli.StringFlag{
			Name:  "state",
			Usage: dealStateUsage(),
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := DaemonContext(cctx)
		proposalCid, err := cid.Decode(cctx.String("proposalcid"))
		if err != nil {
			return err
		}
		var isParamOk bool
		var state storagemarket.StorageDealStatus
		var pieceStatus market.PieceStatus

		if cctx.IsSet("state") {
			isParamOk = true
			state = storageprovider.StringToStorageState[cctx.String("state")]
		}

		if cctx.IsSet("piece-state") {
			pieceStatus = market.PieceStatus(cctx.String("piece-state"))
			isParamOk = true
		}

		if !isParamOk {
			return fmt.Errorf("must set 'state' or 'piece-state'")
		}

		if !cctx.Bool("really-do-it") {
			fmt.Println("Pass --really-do-it to actually execute this action")
			return nil
		}

		return api.UpdateStorageDealStatus(ctx, proposalCid, state, pieceStatus)
	},
}

var dealsPendingPublish = &cli.Command{
	Name:  "pending-publish",
	Usage: "list deals waiting in publish queue",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "publish-now",
			Usage: "send a publish message now",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		if cctx.Bool("publish-now") {
			if err := api.MarketPublishPendingDeals(ctx); err != nil {
				return fmt.Errorf("publishing deals: %w", err)
			}
			fmt.Println("triggered deal publishing")
			return nil
		}

		pendings, err := api.MarketPendingDeals(ctx)
		if err != nil {
			return fmt.Errorf("getting pending deals: %w", err)
		}

		for _, pending := range pendings {
			if len(pending.Deals) > 0 {
				endsIn := time.Until(pending.PublishPeriodStart.Add(pending.PublishPeriod))
				w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
				_, _ = fmt.Fprintf(w, "Publish period:             %s (ends in %s)\n", pending.PublishPeriod, endsIn.Round(time.Second))
				_, _ = fmt.Fprintf(w, "First deal queued at:       %s\n", pending.PublishPeriodStart)
				_, _ = fmt.Fprintf(w, "Deals will be published at: %s\n", pending.PublishPeriodStart.Add(pending.PublishPeriod))
				_, _ = fmt.Fprintf(w, "%d deals queued to be published:\n", len(pending.Deals))
				_, _ = fmt.Fprintf(w, "ProposalCID\tClient\tSize\n")
				for _, deal := range pending.Deals {
					proposalNd, err := cborutil.AsIpld(&deal) // nolint
					if err != nil {
						return err
					}

					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", proposalNd.Cid(), deal.Proposal.Client, units.BytesSize(float64(deal.Proposal.PieceSize)))
				}
				return w.Flush()
			}
		}

		fmt.Println("No deals queued to be published")
		return nil
	},
}

func outputStorageDeals(out io.Writer, deals []market.MinerDeal, verbose bool) error {
	sort.Slice(deals, func(i, j int) bool {
		return deals[i].CreationTime.Time().Before(deals[j].CreationTime.Time())
	})

	w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)

	if verbose {
		_, _ = fmt.Fprintf(w, "Creation\tVerified\tProposalCid\tDealId\tState\tPieceState\tClient\tProvider\tSize\tPrice\tDuration\tTransferChannelID\tAddFundCid\tPublishCid\tMessage\n")
	} else {
		_, _ = fmt.Fprintf(w, "ProposalCid\tDealId\tState\tPieceState\tClient\tProvider\tSize\tPrice\tDuration\n")
	}

	for _, deal := range deals {
		propcid := deal.ProposalCid.String()
		if !verbose {
			propcid = "..." + propcid[len(propcid)-8:]
		}

		fil := types.FIL(types.BigMul(deal.Proposal.StoragePricePerEpoch, types.NewInt(uint64(deal.Proposal.Duration()))))

		if verbose {
			_, _ = fmt.Fprintf(w, "%s\t%t\t", deal.CreationTime.Time().Format(time.Stamp), deal.Proposal.VerifiedDeal)
		}

		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s", propcid, deal.DealID, storagemarket.DealStates[deal.State], deal.PieceStatus,
			deal.Proposal.Client, deal.Proposal.Provider, units.BytesSize(float64(deal.Proposal.PieceSize)), fil, deal.Proposal.Duration())
		if verbose {
			tchid := ""
			if deal.TransferChannelID != nil {
				tchid = deal.TransferChannelID.String()
			}

			addFundcid := ""
			if deal.AddFundsCid != nil {
				addFundcid = deal.AddFundsCid.String()
			}

			pubcid := ""
			if deal.PublishCid != nil {
				pubcid = deal.PublishCid.String()
			}

			_, _ = fmt.Fprintf(w, "\t%s", tchid)
			_, _ = fmt.Fprintf(w, "\t%s", addFundcid)
			_, _ = fmt.Fprintf(w, "\t%s", pubcid)
			_, _ = fmt.Fprintf(w, "\t%s", deal.Message)
		}

		_, _ = fmt.Fprintln(w)
	}

	return w.Flush()
}
