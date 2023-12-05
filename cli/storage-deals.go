package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	tm "github.com/buger/goterm"
	"github.com/docker/go-units"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/ipfs-force-community/droplet/v2/storageprovider"

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
		dealsBatchImportDataCmd,
		importDealCmd,
		dealsListCmd,
		updateStorageDealStateCmd,
		dealsPendingPublish,
		getDealCmd,
		dealStateCmd,
	},
}

var dealsImportDataCmd = &cli.Command{
	Name:  "import-data",
	Usage: "Manually import data for a deal",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "skip-commp",
			Usage: "skip calculate the piece-cid, please use with caution",
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
		},
	},
	ArgsUsage: "<proposal CID or uuid> <file>",
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

		ref := market.ImportDataRef{
			File: cctx.Args().Get(1),
		}

		propCid, err := cid.Decode(cctx.Args().Get(0))
		if err == nil {
			ref.ProposalCID = propCid
		} else {
			ref.UUID, err = uuid.Parse(cctx.Args().Get(0))
			if err != nil {
				return err
			}
		}

		var skipCommP bool
		if cctx.IsSet("skip-commp") {
			if !cctx.IsSet("really-do-it") {
				return fmt.Errorf("pass --really-do-it to actually execute this action")
			}
			skipCommP = true
		}

		if err := api.DealsImportData(ctx, ref, skipCommP); err != nil {
			return err
		}
		fmt.Println("import data success")

		return nil
	},
}

var dealsBatchImportDataCmd = &cli.Command{
	Name:  "batch-import-data",
	Usage: "Batch import data for deals",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "proposals",
			Usage: "proposal cid(or uuid) and car file, eg. --proposals <proposal_cid>,<path_to_car_file>  --proposals <uuid>,<path_to_car_file>",
		},
		&cli.StringFlag{
			Name: "manifest",
			Usage: `A file containing proposal cid and piece cid, eg.
proposalCID,pieceCID
baadfdxxx,badddxxx
basdefxxx,baefaxxx
or
uuid,pieceCID
baad-xxx,badddxxx
bass-xxx,baefaxxx
`,
		},
		&cli.BoolFlag{
			Name:  "skip-commp",
			Usage: "skip calculate the piece-cid, please use with caution",
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
		},
		&cli.StringFlag{
			Name:  "car-dir",
			Usage: "Directory of car files",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := DaemonContext(cctx)

		var proposalFiles []string
		var refs []*market.ImportDataRef
		if cctx.IsSet("proposals") {
			proposalFiles = cctx.StringSlice("proposals")
		} else {
			manifest := cctx.String("manifest")
			if len(manifest) == 0 {
				return fmt.Errorf("must pass proposals or manifest")
			}
			data, err := os.ReadFile(manifest)
			if err != nil {
				return err
			}
			proposalFiles = strings.Split(string(data), "\n")
		}
		carDir := cctx.String("car-dir")
		for _, proposalFile := range proposalFiles {
			arr := strings.Split(proposalFile, ",")
			if len(arr) != 2 {
				continue
			}

			ref := &market.ImportDataRef{
				File: arr[1],
			}
			if len(arr[1]) != 0 && len(carDir) != 0 {
				ref.File = filepath.Join(carDir, ref.File)
			}

			proposalCID, err := cid.Parse(arr[0])
			if err == nil {
				ref.ProposalCID = proposalCID
				refs = append(refs, ref)
				continue
			}

			id, err := uuid.Parse(arr[0])
			if err == nil {
				ref.UUID = id
				refs = append(refs, ref)
			}
		}

		var skipCommP bool
		if cctx.IsSet("skip-commp") {
			if !cctx.IsSet("really-do-it") {
				return fmt.Errorf("pass --really-do-it to actually execute this action")
			}
			skipCommP = true
		}
		res, err := api.DealsBatchImportData(ctx, market.ImportDataRefs{
			Refs:      refs,
			SkipCommP: skipCommP,
		})
		if err != nil {
			return err
		}

		for _, r := range res {
			if len(r.Message) == 0 {
				fmt.Printf("import data success: %s\n", r.Target)
			} else {
				fmt.Printf("import data failed, deal: %s, error: %s\n", r.Target, r.Message)
			}
		}

		return nil
	},
}

var importDealCmd = &cli.Command{
	Name:      "import-deal",
	Usage:     "Manually import lotus-miner or boost deals",
	ArgsUsage: "<deal file>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "Where the order comes from, lotus-miner or boost",
			Value: "lotus-miner",
		},
		&cli.StringSliceFlag{
			Name:  "car-dirs",
			Usage: "directory of car files",
		},
		&cli.Uint64SliceFlag{
			Name: "states",
			Usage: `
What status deal is expected to be imported, default import StorageDealActive and StorageDealWaitingForData deal.
use './droplet storage deal states' to show all states.
part states:
7  StorageDealActive
18 StorageDealWaitingForData
`,
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		fapi, fcloser, err := NewFullNode(cctx, OldMarketRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := DaemonContext(cctx)

		if cctx.Args().Len() < 1 {
			return fmt.Errorf("must specify the path of json file which records the deal")
		}

		fpath := cctx.Args().Get(0)
		data, err := os.ReadFile(fpath)
		if err != nil {
			return fmt.Errorf("read deal file(%s) failed: %v", fpath, err)
		}
		var r result
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}

		expectStates := map[uint64]struct{}{
			storagemarket.StorageDealWaitingForData: {},
			storagemarket.StorageDealActive:         {},
		}
		if cctx.IsSet("states") {
			expectStates = make(map[uint64]struct{})
			for _, v := range cctx.Uint64Slice("states") {
				expectStates[v] = struct{}{}
			}
		}

		getMinerPeer := getMinerPeerFunc(ctx, fapi)
		getPayloadSize := getPayloadSizeFunc(cctx.StringSlice("car-dirs"))
		deals := make([]*market.MinerDeal, 0)
		if cctx.String("from") == "boost" {
			for _, deal := range r.BoostResult.Deals.Deals {
				d, err := deal.minerDeal()
				if err != nil {
					fmt.Printf("parse %s deal failed: %v\n", deal.SignedProposalCid, err)
					continue
				}
				if _, ok := expectStates[d.State]; !ok {
					continue
				}
				d.Miner = getMinerPeer(d.Proposal.Provider)

				if d.PayloadSize == 0 {
					d.PayloadSize = getPayloadSize(d.Proposal.PieceCID)
					d.Ref.RawBlockSize = d.PayloadSize
					if d.PayloadSize == 0 {
						fmt.Printf("deal %s payload size %d\n", deal.SignedProposalCid, d.PayloadSize)
						continue
					}
				}
				deals = append(deals, d)
			}
		} else if cctx.String("from") == "lotus-miner" {
			for _, d := range r.Result {
				if _, ok := expectStates[d.State]; ok {
					d.PayloadSize = d.Ref.RawBlockSize
					d.PieceStatus = market.Undefine
					if d.Ref == nil {
						d.Ref = &storagemarket.DataRef{TransferType: "import"}
					} else {
						d.Ref.TransferType = "import"
					}
					if d.State == storagemarket.StorageDealActive || d.State == storagemarket.StorageDealSlashed {
						d.PieceStatus = market.Proving
					}
					if d.SlashEpoch == 0 {
						d.SlashEpoch = -1
					}
					deals = append(deals, d)
				}
			}
		} else {
			return fmt.Errorf("the value of --from can only be 'lotus-miner' or 'boost' ")
		}
		if err := api.DealsImport(ctx, deals); err != nil {
			return fmt.Errorf("\nimport deals failed: %v", err)
		}
		fmt.Printf("import %d deals success\n", len(deals))

		return nil
	},
}

var offsetFlag = &cli.IntFlag{
	Name:  "offset",
	Usage: "Number of skipped items",
}
var limitFlag = &cli.IntFlag{
	Name:  "limit",
	Value: 20,
	Usage: "Number of entries per page",
}

var dealsListCmd = &cli.Command{
	Name:  "list",
	Usage: "List all deals for this miner",
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
deal state, show all deal state: ./droplet storage deal states.
part states:
8  StorageDealExpired
9  StorageDealSlashed
10 StorageDealRejecting
11 StorageDealFailing
29 StorageDealAwaitingPreCommit
`,
		},
		&cli.StringFlag{
			Name:  "client",
			Usage: "client peer id",
		},
		&cli.BoolFlag{
			Name:  "discard-failed",
			Usage: "filter failed deal, include failing deal, slashed deal, expired deal, error deal",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
		},
		&cli.BoolFlag{
			Name:  "watch",
			Usage: "watch deal updates in real-time, rather than a one time list",
		},
		&cli.BoolFlag{
			Name:  "oldest",
			Usage: "sort by oldest first",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output deal info as json format",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		mAddr := address.Undef
		if cctx.IsSet("miner") {
			mAddr, err = address.NewFromString(cctx.String("miner"))
			if err != nil {
				return fmt.Errorf("para `miner` is invalid: %w", err)
			}
		}

		var statePtr *uint64
		if cctx.IsSet("state") {
			state := cctx.Uint64("state")
			statePtr = &state
		}
		params := market.StorageDealQueryParams{
			Miner:             mAddr,
			State:             statePtr,
			Client:            cctx.String("client"),
			DiscardFailedDeal: cctx.Bool("discard-failed"),
			Page: market.Page{
				Offset: cctx.Int(offsetFlag.Name),
				Limit:  cctx.Int(limitFlag.Name),
			},
		}
		if cctx.Bool("oldest") {
			params.Asc = true
		}

		ctx := ReqContext(cctx)
		deals, err := api.MarketListIncompleteDeals(ctx, &params)
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

				err = outputStorageDeals(tm.Output, deals, verbose, cctx.Bool("json"))
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

		return outputStorageDeals(os.Stdout, deals, verbose, cctx.Bool("json"))
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

var getDealCmd = &cli.Command{
	Name:  "get",
	Usage: "Print a storage deal",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "deal-id",
			Usage: "deal id assign by chain, eg. 1",
		},
		&cli.StringFlag{
			Name:  "proposal-cid",
			Usage: "cid of deal proposal",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output deal info as json format",
		},
	},
	Action: func(cliCtx *cli.Context) error {
		if !cliCtx.IsSet("deal-id") && !cliCtx.IsSet("proposal-cid") {
			return fmt.Errorf("must specify deal id or proposal cid")
		}
		var dealID abi.DealID
		var proposalCid cid.Cid

		if cliCtx.IsSet("deal-id") {
			dealID = abi.DealID(cliCtx.Int64("deal-id"))
		} else {
			var err error
			proposalCid, err = cid.Decode(cliCtx.String("proposal-cid"))
			if err != nil {
				return err
			}
		}

		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cliCtx)

		var deal *market.MinerDeal
		if cliCtx.IsSet("deal-id") {
			deals, err := api.MarketListIncompleteDeals(ctx, &market.StorageDealQueryParams{
				DealID: dealID,
				Page: market.Page{
					Limit: 1,
				},
			})
			if err != nil {
				return err
			}
			if len(deals) == 0 {
				return fmt.Errorf("deal %d not found", dealID)
			}
			deal = &deals[0]
		} else {
			deal, err = api.MarketGetDeal(ctx, proposalCid)
			if err != nil {
				return err
			}
		}

		if cliCtx.Bool("json") {
			data, err := json.MarshalIndent(deal, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		return outputStorageDeal(deal)
	},
}

var dealStateCmd = &cli.Command{
	Name:  "states",
	Usage: "Print all storage deal state",
	Action: func(cliCtx *cli.Context) error {
		return printStates(storagemarket.DealStates)
	},
}

func printStates(data interface{}) error {
	type item struct {
		k int
		v string
	}

	var items []item
	var kvs []kv
	var maxLen int

	switch d := data.(type) {
	case map[uint64]string:
		for k, v := range d {
			items = append(items, item{
				k: int(k),
				v: v,
			})
		}
	case map[retrievalmarket.DealStatus]string:
		for k, v := range d {
			items = append(items, item{
				k: int(k),
				v: v,
			})
		}
	default:
		return fmt.Errorf("unexpected type %T", data)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].k < items[j].k
	})

	for _, item := range items {
		k := fmt.Sprintf("%d", item.k)
		kvs = append(kvs, kv{
			k: k,
			v: item.v,
		})
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}

	fillSpaceAndPrint(kvs, maxLen)

	return nil
}

func outputStorageDeals(out io.Writer, deals []market.MinerDeal, verbose bool, inJson bool) error {
	sort.Slice(deals, func(i, j int) bool {
		return deals[i].CreationTime.Time().Before(deals[j].CreationTime.Time())
	})

	if inJson {
		data, err := json.MarshalIndent(deals, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}

	w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)

	if verbose {
		_, _ = fmt.Fprintf(w, "Creation\tVerified\tProposalCid\tDealId\tState\tPieceState\tClient\tProvider\tSize\tPrice\tDuration\tTransferChannelID\tAddFundCid\tPublishCid\tMessage\n")
	} else {
		_, _ = fmt.Fprintf(w, "ProposalCid\tDealId\tState\tPieceState\tClient\tProvider\tSize\tPrice\tDuration\nID\n")
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

		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s", propcid, deal.DealID, storagemarket.DealStates[deal.State], deal.PieceStatus,
			deal.Proposal.Client, deal.Proposal.Provider, units.BytesSize(float64(deal.Proposal.PieceSize)), fil, deal.Proposal.Duration(), deal.ID)
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

type kv struct {
	k string
	v interface{}
}

func outputStorageDeal(deal *market.MinerDeal) error {
	var err error
	var transferChannelID, label, addFundsCid, publishCid string

	if deal.TransferChannelID != nil {
		transferChannelID = deal.TransferChannelID.String()
	}
	label, err = deal.Proposal.Label.ToString()
	if err != nil {
		return err
	}
	dataCid, err := cid.Decode(label)
	if err != nil {
		return fmt.Errorf("parse %s to cid failed: %v", label, err)
	}
	if deal.AddFundsCid != nil {
		addFundsCid = deal.AddFundsCid.String()
	}
	if deal.PublishCid != nil {
		publishCid = deal.PublishCid.String()
	}
	fil := types.FIL(types.BigMul(deal.Proposal.StoragePricePerEpoch, types.NewInt(uint64(deal.Proposal.Duration()))))

	data := []kv{
		{"Creation", deal.CreationTime.Time().Format(time.RFC3339)},
		{"State", storagemarket.DealStates[deal.State]},
		{"VerifiedDeal", deal.Proposal.VerifiedDeal},
		{"DealID", deal.DealID},
		{"PieceCID", deal.Proposal.PieceCID},
		{"PieceStatus", deal.PieceStatus},
		{"Provider", deal.Proposal.Provider},
		{"PieceSize", units.BytesSize(float64(deal.Proposal.PieceSize))},
		{"Price", fil},
		{"Duration", deal.Proposal.Duration()},
		{"Offset", deal.Offset},
		{"Client", deal.Proposal.Client},
		{"TransferID", transferChannelID},
		{"AddFundsCid", addFundsCid},
		{"PublishCid", publishCid},
		{"Message", deal.Message},
		{"TransferType", deal.Ref.TransferType},
		{"PayloadCID", deal.Ref.Root},
		{"PayloadSize", deal.PayloadSize},
		{"StartEpoch", deal.Proposal.StartEpoch},
		{"EndEpoch", deal.Proposal.EndEpoch},
		{"SlashEpoch", deal.SlashEpoch},
		{"StoragePricePerEpoch", deal.Proposal.StoragePricePerEpoch},
		{"ProviderCollateral", deal.Proposal.ProviderCollateral},
		{"ClientCollateral", deal.Proposal.ClientCollateral},
		{"Label", dataCid},
		{"MinerPeerID", deal.Miner.Pretty()},
		{"ClientPeerID", deal.Client.Pretty()},
		{"FundsReserved", deal.FundsReserved},
		{"AvailableForRetrieval", deal.AvailableForRetrieval},
		{"SectorNumber", deal.SectorNumber},
		{"PiecePath", deal.PiecePath},
		{"MetadataPath", deal.MetadataPath},
		{"FastRetrieval", deal.FastRetrieval},
		{"InboundCAR", deal.InboundCAR},
		{"UpdatedAt", time.Unix(int64(deal.UpdatedAt), 0).Format(time.RFC3339)},
		{"ID", deal.ID},
	}

	fillSpaceAndPrint(data, len("AvailableForRetrieval"))

	return nil
}

func fillSpaceAndPrint(data []kv, maxLen int) {
	for _, d := range data {
		for i := len(d.k); i < maxLen; i++ {
			d.k += " "
		}
		fmt.Println(d.k, d.v)
	}
}
