package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	tm "github.com/buger/goterm"
	"github.com/docker/go-units"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus-market/v2/storageprovider"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
)

var storageDealSelectionCmd = &cli.Command{
	Name:  "selection",
	Usage: "Configure acceptance criteria for storage deal proposals",
	Flags: []cli.Flag{
		minerFlag,
	},
	Subcommands: []*cli.Command{
		storageDealSelectionShowCmd,
		storageDealSelectionResetCmd,
		storageDealSelectionRejectCmd,
	},
}

var storageDealSelectionShowCmd = &cli.Command{
	Name:  "list",
	Usage: "List storage deal proposal selection criteria",
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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
	Name:  "reset",
	Usage: "Reset storage deal proposal selection criteria to default values",
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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
	Name:  "reject",
	Usage: "Configure criteria which necessitate automatic rejection",
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
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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

var setAskCmd = &cli.Command{
	Name:  "set-ask",
	Usage: "Configure the miner's ask",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "price",
			Usage:    "Set the price of the ask for unverified deals (specified as FIL / GiB / Epoch) to `PRICE`.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "verified-price",
			Usage:    "Set the price of the ask for verified deals (specified as FIL / GiB / Epoch) to `PRICE`",
			Required: true,
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
		},
		&cli.StringFlag{
			Name:     "miner",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

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

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return fmt.Errorf("para `miner` is invalid: %w", err)
		}

		ssize, err := api.ActorSectorSize(ctx, maddr)
		if err != nil {
			return err
		}

		smax := int64(ssize)

		if max == 0 {
			max = smax
		}

		if max > smax {
			return fmt.Errorf("max piece size (w/bit-padding) %s cannot exceed miner sector size %s", types.SizeStr(types.NewInt(uint64(max))), types.SizeStr(types.NewInt(uint64(smax))))
		}

		return api.MarketSetAsk(ctx, maddr, types.BigInt(pri), types.BigInt(vpri), abi.ChainEpoch(qty), abi.PaddedPieceSize(min), abi.PaddedPieceSize(max))
	},
}

var getAskCmd = &cli.Command{
	Name:  "get-ask",
	Usage: "Print the miner's ask",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "miner",
			Required: true,
		},
	},
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

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return fmt.Errorf("para `miner` is invalid: %w", err)
		}

		sask, err := smapi.MarketGetAsk(ctx, maddr)
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

var StorageDealsCmd = &cli.Command{
	Name:  "storage-deals",
	Usage: "Manage storage deals and related configuration",
	Subcommands: []*cli.Command{
		dealsImportDataCmd,
		importOfflineDealCmd,
		dealsListCmd,
		updateStorageDealStateCmd,
		storageDealSelectionCmd,
		setAskCmd,
		getAskCmd,
		setBlocklistCmd,
		getBlocklistCmd,
		resetBlocklistCmd,
		expectedSealDurationCmd,
		maxDealStartDelayCmd,
		dealsPublishMsgPeriodCmd,
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

var getBlocklistCmd = &cli.Command{
	Name:  "get-blocklist",
	Usage: "List the contents of the miner's piece CID blocklist",
	Flags: []cli.Flag{
		&CidBaseFlag,
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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
	Name:      "set-blocklist",
	Usage:     "Set the miner's list of blocklisted piece CIDs",
	ArgsUsage: "[<path-of-file-containing-newline-delimited-piece-CIDs> (optional, will read from stdin if omitted)]",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		scanner := bufio.NewScanner(os.Stdin)
		if cctx.Args().Present() && cctx.Args().First() != "-" {
			absPath, err := filepath.Abs(cctx.Args().First())
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
	Name:  "reset-blocklist",
	Usage: "Remove all entries from the miner's piece CID blocklist",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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

var expectedSealDurationCmd = &cli.Command{
	Name:  "seal-duration",
	Usage: "Configure the expected time, that you expect sealing sectors to take. Deals that start before this duration will be rejected.",
	Flags: []cli.Flag{
		minerFlag,
	},
	Subcommands: []*cli.Command{
		expectedSealDurationGetCmd,
		expectedSealDurationSetCmd,
	},
}

var expectedSealDurationGetCmd = &cli.Command{
	Name: "get",
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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
	Name:      "set-seal-duration",
	Usage:     "eg. '1m','30s',...",
	ArgsUsage: "<duration>",
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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

		d, err := time.ParseDuration(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}

		return marketApi.SectorSetExpectedSealDuration(ctx, mAddr, d)
	},
}

var maxDealStartDelayCmd = &cli.Command{
	Name:  "max-start-delay",
	Usage: "Configure the maximum amount of time proposed deal StartEpoch can be in future.",
	Flags: []cli.Flag{
		minerFlag,
	},
	Subcommands: []*cli.Command{
		maxDealStartDelayGetCmd,
		maxDealStartDelaySetCmd,
	},
}

var maxDealStartDelayGetCmd = &cli.Command{
	Name: "get",
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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
	ArgsUsage: "<minutes>",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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

		delay, err := time.ParseDuration(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}

		return marketApi.DealsSetMaxStartDelay(ctx, mAddr, delay)
	},
}

var dealsPublishMsgPeriodCmd = &cli.Command{
	Name:  "max-start-delay",
	Usage: "Configure the the amount of time to wait for more deals to be ready to publish before publishing them all as a batch.",
	Flags: []cli.Flag{
		minerFlag,
	},
	Subcommands: []*cli.Command{
		dealsPublishMsgPeriodGetCmd,
		dealsPublishMsgPeriodSetCmd,
	},
}

var dealsPublishMsgPeriodGetCmd = &cli.Command{
	Name: "get",
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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
	ArgsUsage: "<duration>",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
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

		period, err := time.ParseDuration(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}
		return marketApi.DealsSetPublishMsgPeriod(ctx, mAddr, period)
	},
}

var DataTransfersCmd = &cli.Command{
	Name:  "data-transfers",
	Usage: "Manage data transfers",
	Subcommands: []*cli.Command{
		transfersListCmd,
		marketRestartTransfer,
		marketCancelTransfer,
	},
}

var marketRestartTransfer = &cli.Command{
	Name:  "restart",
	Usage: "Force restart a stalled data transfer",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "peerid",
			Usage: "narrow to transfer with specific peer",
		},
		&cli.BoolFlag{
			Name:  "initiator",
			Usage: "specify only transfers where peer is/is not initiator",
			Value: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return cli.ShowCommandHelp(cctx, cctx.Command.Name)
		}
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		transferUint, err := strconv.ParseUint(cctx.Args().First(), 10, 64)
		if err != nil {
			return fmt.Errorf("Error reading transfer ID: %w", err)
		}
		transferID := datatransfer.TransferID(transferUint)
		initiator := cctx.Bool("initiator")
		var other peer.ID
		if pidstr := cctx.String("peerid"); pidstr != "" {
			p, err := peer.Decode(pidstr)
			if err != nil {
				return err
			}
			other = p
		} else {
			channels, err := nodeApi.MarketListDataTransfers(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, channel := range channels {
				if channel.IsInitiator == initiator && channel.TransferID == transferID {
					other = channel.OtherPeer
					found = true
					break
				}
			}
			if !found {
				return errors.New("unable to find matching data transfer")
			}
		}

		return nodeApi.MarketRestartDataTransfer(ctx, transferID, other, initiator)
	},
}

var marketCancelTransfer = &cli.Command{
	Name:  "cancel",
	Usage: "Force cancel a data transfer",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "peerid",
			Usage: "narrow to transfer with specific peer",
		},
		&cli.BoolFlag{
			Name:  "initiator",
			Usage: "specify only transfers where peer is/is not initiator",
			Value: false,
		},
		&cli.DurationFlag{
			Name:  "cancel-timeout",
			Usage: "time to wait for cancel to be sent to client",
			Value: 5 * time.Second,
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return cli.ShowCommandHelp(cctx, cctx.Command.Name)
		}
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		transferUint, err := strconv.ParseUint(cctx.Args().First(), 10, 64)
		if err != nil {
			return fmt.Errorf("Error reading transfer ID: %w", err)
		}
		transferID := datatransfer.TransferID(transferUint)
		initiator := cctx.Bool("initiator")
		var other peer.ID
		if pidstr := cctx.String("peerid"); pidstr != "" {
			p, err := peer.Decode(pidstr)
			if err != nil {
				return err
			}
			other = p
		} else {
			channels, err := nodeApi.MarketListDataTransfers(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, channel := range channels {
				if channel.IsInitiator == initiator && channel.TransferID == transferID {
					other = channel.OtherPeer
					found = true
					break
				}
			}
			if !found {
				return errors.New("unable to find matching data transfer")
			}
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, cctx.Duration("cancel-timeout"))
		defer cancel()
		return nodeApi.MarketCancelDataTransfer(timeoutCtx, transferID, other, initiator)
	},
}

var transfersListCmd = &cli.Command{
	Name:  "list",
	Usage: "List ongoing data transfers for this miner",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "print verbose transfer details",
		},
		&cli.BoolFlag{
			Name:  "color",
			Usage: "use color in display output",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "completed",
			Usage: "show completed data transfers",
		},
		&cli.BoolFlag{
			Name:  "watch",
			Usage: "watch deal updates in real-time, rather than a one time list",
		},
		&cli.BoolFlag{
			Name:  "show-failed",
			Usage: "show failed/cancelled transfers",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		channels, err := api.MarketListDataTransfers(ctx)
		if err != nil {
			return err
		}

		verbose := cctx.Bool("verbose")
		completed := cctx.Bool("completed")
		color := cctx.Bool("color")
		watch := cctx.Bool("watch")
		showFailed := cctx.Bool("show-failed")
		if watch {
			channelUpdates, err := api.MarketDataTransferUpdates(ctx)
			if err != nil {
				return err
			}

			for {
				tm.Clear() // Clear current screen

				tm.MoveCursor(1, 1)

				OutputDataTransferChannels(tm.Screen, channels, verbose, completed, color, showFailed)

				tm.Flush()

				select {
				case <-ctx.Done():
					return nil
				case channelUpdate := <-channelUpdates:
					var found bool
					for i, existing := range channels {
						if existing.TransferID == channelUpdate.TransferID &&
							existing.OtherPeer == channelUpdate.OtherPeer &&
							existing.IsSender == channelUpdate.IsSender &&
							existing.IsInitiator == channelUpdate.IsInitiator {
							channels[i] = channelUpdate
							found = true
							break
						}
					}
					if !found {
						channels = append(channels, channelUpdate)
					}
				}
			}
		}
		OutputDataTransferChannels(os.Stdout, channels, verbose, completed, color, showFailed)
		return nil
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
