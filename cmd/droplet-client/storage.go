package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	tm "github.com/buger/goterm"
	"github.com/chzyer/readline"
	"github.com/docker/go-units"
	"github.com/fatih/color"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/venus-shared/types/market/client"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/cli/tablewriter"
)

var storageCmd = &cli.Command{
	Name:        "storage",
	Usage:       "manage storage deals",
	Description: `Store customers query the ask list in the network, and select the satisfactory one for storage transactions.`,
	Subcommands: []*cli.Command{
		storageAsksCmd,
		storageDealsCmd,
	},
}

// TODO ???
// Actor consts
// TODO: pieceSize unused from actors
// nolint
var MinDealDuration, MaxDealDuration = policy.DealDurationBounds(0)

const BlockDelaySecs = uint64(builtin.EpochDurationSeconds)

type QueriedAsk struct {
	Ask  *storagemarket.StorageAsk
	Ping time.Duration
}

func GetAsks(ctx context.Context, api v1api.FullNode, capi clientapi.IMarketClient) ([]QueriedAsk, error) {
	isTTY := true
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		isTTY = false
	}
	if isTTY {
		color.Blue(".. getting miner list")
	}
	miners, err := api.StateListMiners(ctx, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("getting miner list: %w", err)
	}

	var lk sync.Mutex
	var found int64
	var withMinPower []address.Address
	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		wg.Add(len(miners))

		throttle := make(chan struct{}, 50)
		for _, miner := range miners {
			throttle <- struct{}{}
			go func(miner address.Address) {
				defer wg.Done()
				defer func() {
					<-throttle
				}()

				power, err := api.StateMinerPower(ctx, miner, types.EmptyTSK)
				if err != nil {
					return
				}

				if power.HasMinPower { // TODO: Lower threshold
					atomic.AddInt64(&found, 1)
					lk.Lock()
					withMinPower = append(withMinPower, miner)
					lk.Unlock()
				}
			}(miner)
		}

		wg.Wait()
	}()

loop:
	for {
		select {
		case <-time.After(150 * time.Millisecond):
			if isTTY {
				fmt.Printf("\r* Found %d miners with power", atomic.LoadInt64(&found))
			}
		case <-done:
			break loop
		}
	}
	if isTTY {
		fmt.Printf("\r* Found %d miners with power\n", atomic.LoadInt64(&found))

		color.Blue(".. querying asks")
	}

	var asks []QueriedAsk
	var queried, got int64

	done = make(chan struct{})
	go func() {
		defer close(done)

		var wg sync.WaitGroup
		wg.Add(len(withMinPower))

		throttle := make(chan struct{}, 50)
		for _, miner := range withMinPower {
			throttle <- struct{}{}
			go func(miner address.Address) {
				defer wg.Done()
				defer func() {
					<-throttle
					atomic.AddInt64(&queried, 1)
				}()

				ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
				defer cancel()

				mi, err := api.StateMinerInfo(ctx, miner, types.EmptyTSK)
				if err != nil {
					return
				}
				if mi.PeerId == nil {
					return
				}

				ask, err := capi.ClientQueryAsk(ctx, *mi.PeerId, miner)
				if err != nil {
					return
				}

				rt := time.Now()
				_, err = capi.ClientQueryAsk(ctx, *mi.PeerId, miner)
				if err != nil {
					return
				}
				pingDuration := time.Since(rt)

				atomic.AddInt64(&got, 1)
				lk.Lock()
				asks = append(asks, QueriedAsk{
					Ask:  ask,
					Ping: pingDuration,
				})
				lk.Unlock()
			}(miner)
		}

		wg.Wait()
	}()

loop2:
	for {
		select {
		case <-time.After(150 * time.Millisecond):
			if isTTY {
				fmt.Printf("\r* Queried %d asks, got %d responses", atomic.LoadInt64(&queried), atomic.LoadInt64(&got))
			}
		case <-done:
			break loop2
		}
	}
	if isTTY {
		fmt.Printf("\r* Queried %d asks, got %d responses\n", atomic.LoadInt64(&queried), atomic.LoadInt64(&got))
	}

	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Ask.Price.LessThan(asks[j].Ask.Price)
	})

	return asks, nil
}

var storageAsksCmd = &cli.Command{
	Name:  "asks",
	Usage: "manage storage asks",
	Subcommands: []*cli.Command{
		storageAsksQueryCmd,
		storageAsksListCmd,
	},
}

var storageAsksQueryCmd = &cli.Command{
	Name:      "query",
	Usage:     "Find a miners ask",
	ArgsUsage: "[minerAddress]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "peerid",
			Usage: "specify peer ID of node to make query against",
		},
		&cli.Int64Flag{
			Name:  "size",
			Usage: "data size in bytes",
		},
		&cli.Int64Flag{
			Name:  "duration",
			Usage: "deal duration",
		},
	},
	Action: func(cctx *cli.Context) error {
		afmt := cli2.NewAppFmt(cctx.App)
		if cctx.NArg() != 1 {
			afmt.Println("Usage: query-ask [minerAddress]")
			return nil
		}

		maddr, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()

		var pid peer.ID
		if pidstr := cctx.String("peerid"); pidstr != "" {
			p, err := peer.Decode(pidstr)
			if err != nil {
				return err
			}
			pid = p
		} else {
			mi, err := fapi.StateMinerInfo(ctx, maddr, types.EmptyTSK)
			if err != nil {
				return fmt.Errorf("failed to get peerID for miner: %w", err)
			}

			if mi.PeerId == nil || *mi.PeerId == peer.ID("SETME") {
				return fmt.Errorf("the miner hasn't initialized yet")
			}

			pid = *mi.PeerId
		}

		ask, err := api.ClientQueryAsk(ctx, pid, maddr)
		if err != nil {
			if strings.Contains(err.Error(), "context by cancel") {
				return fmt.Errorf("timeout: %v", err)
			}
			return err
		}

		afmt.Printf("Ask: %s\n", maddr)
		afmt.Printf("Price per GiB: %s\n", types.FIL(ask.Price))
		afmt.Printf("Verified Price per GiB: %s\n", types.FIL(ask.VerifiedPrice))
		afmt.Printf("Max Piece size: %s\n", types.SizeStr(types.NewInt(uint64(ask.MaxPieceSize))))
		afmt.Printf("Min Piece size: %s\n", types.SizeStr(types.NewInt(uint64(ask.MinPieceSize))))

		size := cctx.Int64("size")
		if size == 0 {
			return nil
		}
		perEpoch := types.BigDiv(types.BigMul(ask.Price, types.NewInt(uint64(size))), types.NewInt(1<<30))
		afmt.Printf("Price per Block: %s\n", types.FIL(perEpoch))

		duration := cctx.Int64("duration")
		if duration == 0 {
			return nil
		}
		afmt.Printf("Total Price: %s\n", types.FIL(types.BigMul(perEpoch, types.NewInt(uint64(duration)))))

		return nil
	},
}

var storageAsksListCmd = &cli.Command{
	Name:  "list",
	Usage: "List asks for top miners",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "by-ping",
			Usage: "sort by ping",
		},
		&cli.StringFlag{
			Name:  "output-format",
			Value: "text",
			Usage: "Either 'text' or 'csv'",
		},
	},
	Action: func(cctx *cli.Context) error {
		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		asks, err := GetAsks(ctx, fapi, api)
		if err != nil {
			return err
		}

		if cctx.Bool("by-ping") {
			sort.Slice(asks, func(i, j int) bool {
				return asks[i].Ping < asks[j].Ping
			})
		}
		pfmt := "%s: min:%s max:%s price:%s/GiB/Epoch verifiedPrice:%s/GiB/Epoch ping:%s\n"
		if cctx.String("output-format") == "csv" {
			fmt.Printf("Miner,Min,Max,Price,VerifiedPrice,Ping\n")
			pfmt = "%s,%s,%s,%s,%s,%s\n"
		}

		for _, a := range asks {
			ask := a.Ask

			fmt.Printf(pfmt, ask.Miner,
				types.SizeStr(types.NewInt(uint64(ask.MinPieceSize))),
				types.SizeStr(types.NewInt(uint64(ask.MaxPieceSize))),
				types.FIL(ask.Price),
				types.FIL(ask.VerifiedPrice),
				a.Ping,
			)
		}

		return nil
	},
}

var storageDealsCmd = &cli.Command{
	Name:  "deals",
	Usage: "storage deals",
	Subcommands: []*cli.Command{
		storageDealsInitCmd,
		storageDelesBatchCmd,
		storageDealsListCmd,
		storageDealsStatsCmd,
		storageDealsGetCmd,
		storageDealsInspectCmd,
		verifiedDealStatsCmd,
		storageDealsExportCmd,
		storageDealInitV2,
		storageDealStatus,
	},
}

var storageDealsInitCmd = &cli.Command{
	Name:  "init",
	Usage: "Initialize storage deal with a miner",
	Description: `Make a deal with a miner.
dataCid comes from running 'droplet-client data import'.
miner is the address of the miner you wish to make a deal with.
price is measured in FIL/Epoch. Miners usually don't accept a bid
lower than their advertised ask (which is in FIL/GiB/Epoch). You can check a miners listed price
with './droplet-client storage asks query <miner address>'.
duration is how long the miner should store the data for, in blocks.
The minimum value is 518400 (6 months).`,
	ArgsUsage: "[dataCid miner price duration]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "manual-piece-cid",
			Usage: "manually specify piece commitment for data (dataCid must be to a car file)",
		},
		&cli.Int64Flag{
			Name:  "manual-piece-size",
			Usage: "if manually specifying piece cid, used to specify size (dataCid must be to a car file)",
		},
		&cli.BoolFlag{
			Name:  "manual-stateless-deal",
			Usage: "instructs the node to send an offline deal without registering it with the deallist/fsm",
		},
		&cli.StringFlag{
			Name:  "from",
			Usage: "specify address to fund the deal with",
		},
		&cli.Int64Flag{
			Name:  "start-epoch",
			Usage: "specify the epoch that the deal should start at",
			Value: -1,
		},
		&cli.BoolFlag{
			Name:  "fast-retrieval",
			Usage: "indicates that data should be available for fast retrieval",
			Value: true,
		},
		&cli.BoolFlag{
			Name:        "verified-deal",
			Usage:       "indicate that the deal counts towards verified client total",
			DefaultText: "true if client is verified, false otherwise",
		},
		&cli.StringFlag{
			Name:  "provider-collateral",
			Usage: "specify the requested provider collateral the miner should put up",
		},
		&cli2.CidBaseFlag,
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return interactiveDeal(cctx)
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)
		afmt := cli2.NewAppFmt(cctx.App)

		if cctx.NArg() != 4 {
			return errors.New("expected 4 args: dataCid, miner, price, duration")
		}

		// [data, miner, price, dur]
		p, err := dealParamsFromContext(cctx, api, fapi, false)
		if err != nil {
			return err
		}

		data, err := cid.Parse(p.firstArg)
		if err != nil {
			return err
		}

		ref := &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         data,
		}

		if mpc := cctx.String("manual-piece-cid"); mpc != "" {
			c, err := cid.Parse(mpc)
			if err != nil {
				return fmt.Errorf("failed to parse provided manual piece cid: %w", err)
			}

			ref.PieceCid = &c

			psize := cctx.Int64("manual-piece-size")
			if psize == 0 {
				return fmt.Errorf("must specify piece size when manually setting cid")
			}

			ref.PieceSize = abi.UnpaddedPieceSize(psize)
			ref.TransferType = storagemarket.TTManual
			p.statelessDeal = true
		}

		sdParams := &client.DealParams{
			Data:               ref,
			Wallet:             p.from,
			Miner:              p.miner[0],
			EpochPrice:         types.BigInt(p.price),
			MinBlocksDuration:  uint64(p.dur),
			DealStartEpoch:     abi.ChainEpoch(cctx.Int64("start-epoch")),
			FastRetrieval:      cctx.Bool("fast-retrieval"),
			VerifiedDeal:       p.isVerified,
			ProviderCollateral: p.provCol,
		}

		if p.isVerified && p.dcap < uint64(sdParams.Data.PieceSize.Padded()) {
			return fmt.Errorf("not enough datacap, need %d, has: %d", p.dcap, sdParams.Data.PieceSize.Padded())
		}

		var proposal *cid.Cid
		if p.statelessDeal {
			if ref.TransferType != storagemarket.TTManual || p.price.Int64() != 0 {
				return errors.New("when manual-stateless-deal is enabled, you must also provide a 'price' of 0 and specify 'manual-piece-cid' and 'manual-piece-size'")
			}
			proposal, err = api.ClientStatelessDeal(ctx, sdParams)
		} else {
			proposal, err = api.ClientStartDeal(ctx, sdParams)
		}

		if err != nil {
			return err
		}

		encoder, err := cli2.GetCidEncoder(cctx)
		if err != nil {
			return err
		}

		afmt.Println("proposal cid: ", encoder.Encode(*proposal))

		return nil
	},
}

func interactiveDeal(cctx *cli.Context) error {
	fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
	if err != nil {
		return err
	}
	defer fcloser()

	api, closer, err := cli2.NewMarketClientNode(cctx)
	if err != nil {
		return err
	}
	defer closer()
	ctx := cli2.ReqContext(cctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	afmt := cli2.NewAppFmt(cctx.App)

	state := "import"
	gib := types.NewInt(1 << 30)

	var data cid.Cid
	var days int
	var maddrs []address.Address
	var ask []storagemarket.StorageAsk
	var epochPrices []big.Int
	var dur time.Duration
	var epochs abi.ChainEpoch
	var verified bool
	var ds client.DataCIDSize

	// find
	var candidateAsks []QueriedAsk
	var budget types.FIL
	var dealCount int64
	var medianPing, maxAcceptablePing time.Duration

	var a address.Address
	if from := cctx.String("from"); from != "" {
		faddr, err := address.NewFromString(from)
		if err != nil {
			return fmt.Errorf("failed to parse 'from' address: %w", err)
		}
		a = faddr
	} else {
		def, err := api.DefaultAddress(ctx)
		if err != nil {
			return err
		}
		a = def
	}

	fromBal, err := fapi.WalletBalance(ctx, a)
	if err != nil {
		return fmt.Errorf("checking from address balance: %w", err)
	}

	printErr := func(err error) {
		afmt.Printf("%s %s\n", color.RedString("Error:"), err.Error())
	}

	cs := readline.NewCancelableStdin(afmt.Stdin)
	go func() {
		<-ctx.Done()
		cs.Close() // nolint:errcheck
	}()

	rl := bufio.NewReader(cs)

uiLoop:
	for {
		// TODO: better exit handling
		if err := ctx.Err(); err != nil {
			return err
		}

		switch state {
		case "import":
			afmt.Print("Data CID (from " + color.YellowString("droplet-client data import") + "): ")

			_cidStr, _, err := rl.ReadLine()
			cidStr := string(_cidStr)
			if err != nil {
				printErr(fmt.Errorf("reading cid string: %w", err))
				continue
			}

			data, err = cid.Parse(cidStr)
			if err != nil {
				printErr(fmt.Errorf("parsing cid string: %w", err))
				continue
			}

			color.Blue(".. calculating data size\n")
			ds, err = api.ClientDealPieceCID(ctx, data)
			if err != nil {
				return err
			}

			fmt.Println("PieceCid: ", ds.PieceCID, " PayLoadSize:", ds.PayloadSize, "PieceSize:", ds.PieceSize)
			state = "duration"
		case "duration":
			afmt.Print("Deal duration (days): ")

			_daystr, _, err := rl.ReadLine()
			daystr := string(_daystr)
			if err != nil {
				return err
			}

			_, err = fmt.Sscan(daystr, &days)
			if err != nil {
				printErr(fmt.Errorf("parsing duration: %w", err))
				continue
			}

			minDealDurationDays := uint64(MinDealDuration) / (builtin.SecondsInDay / BlockDelaySecs)
			if days < int(minDealDurationDays) {
				printErr(fmt.Errorf("minimum duration is %d days", minDealDurationDays))
				continue
			}

			dur = 24 * time.Hour * time.Duration(days)
			epochs = abi.ChainEpoch(dur / (time.Duration(BlockDelaySecs) * time.Second))

			state = "verified"
		case "verified":
			ts, err := fapi.ChainHead(ctx)
			if err != nil {
				return err
			}

			dcap, err := fapi.StateVerifiedClientStatus(ctx, a, ts.Key())
			if err != nil {
				return err
			}

			if dcap == nil {
				state = "miner"
				continue
			}

			if dcap.Uint64() < uint64(ds.PieceSize) {
				color.Yellow(".. not enough DataCap available for a verified deal\n")
				state = "miner"
				continue
			}

			afmt.Print("\nMake this a verified deal? (yes/no): ")

			_yn, _, err := rl.ReadLine()
			yn := string(_yn)
			if err != nil {
				return err
			}

			switch yn {
			case "yes":
				verified = true
			case "no":
				verified = false
			default:
				afmt.Println("Type in full 'yes' or 'no'")
				continue
			}

			state = "miner"
		case "miner":
			afmt.Print("Miner Addresses (f0.. f0..), none to find: ")

			_maddrsStr, _, err := rl.ReadLine()
			maddrsStr := string(_maddrsStr)
			if err != nil {
				printErr(fmt.Errorf("reading miner address: %w", err))
				continue
			}

			maddrs = nil
			for _, s := range strings.Fields(maddrsStr) {
				maddr, err := address.NewFromString(strings.TrimSpace(s))
				if err != nil {
					printErr(fmt.Errorf("parsing miner address: %w", err))
					continue uiLoop
				}

				maddrs = append(maddrs, maddr)
			}

			state = "query"
			if len(maddrs) == 0 {
				state = "find"
			}
		case "find":
			asks, err := GetAsks(ctx, fapi, api)
			if err != nil {
				return err
			}

			if len(asks) == 0 {
				printErr(fmt.Errorf("no asks found"))
				continue uiLoop
			}

			medianPing = asks[len(asks)/2].Ping
			var avgPing time.Duration
			for _, ask := range asks {
				avgPing += ask.Ping
			}
			avgPing /= time.Duration(len(asks))

			for _, ask := range asks {
				if ask.Ask.MinPieceSize > ds.PieceSize {
					continue
				}
				if ask.Ask.MaxPieceSize < ds.PieceSize {
					continue
				}
				candidateAsks = append(candidateAsks, ask)
			}

			afmt.Printf("Found %d candidate asks\n", len(candidateAsks))
			afmt.Printf("Average network latency: %s; Median latency: %s\n", avgPing.Truncate(time.Millisecond), medianPing.Truncate(time.Millisecond))
			state = "max-ping"
		case "max-ping":
			maxAcceptablePing = medianPing

			afmt.Printf("Maximum network latency (default: %s) (ms): ", maxAcceptablePing.Truncate(time.Millisecond))
			_latStr, _, err := rl.ReadLine()
			latStr := string(_latStr)
			if err != nil {
				printErr(fmt.Errorf("reading maximum latency: %w", err))
				continue
			}

			if latStr != "" {
				maxMs, err := strconv.ParseInt(latStr, 10, 64)
				if err != nil {
					printErr(fmt.Errorf("parsing FIL: %w", err))
					continue uiLoop
				}

				maxAcceptablePing = time.Millisecond * time.Duration(maxMs)
			}

			var goodAsks []QueriedAsk
			for _, candidateAsk := range candidateAsks {
				if candidateAsk.Ping < maxAcceptablePing {
					goodAsks = append(goodAsks, candidateAsk)
				}
			}

			if len(goodAsks) == 0 {
				afmt.Printf("no asks left after filtering for network latency\n")
				continue uiLoop
			}

			afmt.Printf("%d asks left after filtering for network latency\n", len(goodAsks))
			candidateAsks = goodAsks

			state = "find-budget"
		case "find-budget":
			afmt.Printf("Proposing from %s, Current Balance: %s\n", a, types.FIL(fromBal))
			afmt.Print("Maximum budget (FIL): ") // TODO: Propose some default somehow?

			_budgetStr, _, err := rl.ReadLine()
			budgetStr := string(_budgetStr)
			if err != nil {
				printErr(fmt.Errorf("reading miner address: %w", err))
				continue
			}

			budget, err = types.ParseFIL(budgetStr)
			if err != nil {
				printErr(fmt.Errorf("parsing FIL: %w", err))
				continue uiLoop
			}

			var goodAsks []QueriedAsk
			for _, ask := range candidateAsks {
				p := ask.Ask.Price
				if verified {
					p = ask.Ask.VerifiedPrice
				}

				epochPrice := types.BigDiv(types.BigMul(p, types.NewInt(uint64(ds.PieceSize))), gib)
				totalPrice := types.BigMul(epochPrice, types.NewInt(uint64(epochs)))

				if totalPrice.LessThan(abi.TokenAmount(budget)) {
					goodAsks = append(goodAsks, ask)
				}
			}
			candidateAsks = goodAsks
			afmt.Printf("%d asks within budget\n", len(candidateAsks))
			state = "find-count"
		case "find-count":
			afmt.Print("Deals to make (1): ")
			dealcStr, _, err := rl.ReadLine()
			if err != nil {
				printErr(fmt.Errorf("reading deal count: %w", err))
				continue
			}

			dealCount, err = strconv.ParseInt(string(dealcStr), 10, 64)
			if err != nil {
				return err
			}

			color.Blue(".. Picking miners")

			// TODO: some better strategy (this tries to pick randomly)
			var pickedAsks []*storagemarket.StorageAsk
		pickLoop:
			for i := 0; i < 64; i++ {
				rand.Shuffle(len(candidateAsks), func(i, j int) {
					candidateAsks[i], candidateAsks[j] = candidateAsks[j], candidateAsks[i]
				})

				remainingBudget := abi.TokenAmount(budget)
				pickedAsks = []*storagemarket.StorageAsk{}

				for _, ask := range candidateAsks {
					p := ask.Ask.Price
					if verified {
						p = ask.Ask.VerifiedPrice
					}

					epochPrice := types.BigDiv(types.BigMul(p, types.NewInt(uint64(ds.PieceSize))), gib)
					totalPrice := types.BigMul(epochPrice, types.NewInt(uint64(epochs)))

					if totalPrice.GreaterThan(remainingBudget) {
						continue
					}

					pickedAsks = append(pickedAsks, ask.Ask)
					remainingBudget = big.Sub(remainingBudget, totalPrice)

					if len(pickedAsks) == int(dealCount) {
						break pickLoop
					}
				}
			}

			for _, pickedAsk := range pickedAsks {
				maddrs = append(maddrs, pickedAsk.Miner)
				ask = append(ask, *pickedAsk)
			}

			state = "confirm"
		case "query":
			color.Blue(".. querying miner asks")

			for _, maddr := range maddrs {
				mi, err := fapi.StateMinerInfo(ctx, maddr, types.EmptyTSK)
				if err != nil {
					printErr(fmt.Errorf("failed to get peerID for miner: %w", err))
					state = "miner"
					continue uiLoop
				}
				if mi.PeerId == nil {
					printErr(fmt.Errorf("not set peer id for miner"))
					state = "miner"
					continue uiLoop
				}
				a, err := api.ClientQueryAsk(ctx, *mi.PeerId, maddr)
				if err != nil {
					printErr(fmt.Errorf("failed to query ask: %w", err))
					state = "miner"
					continue uiLoop
				}

				ask = append(ask, *a)
			}

			// TODO: run more validation
			state = "confirm"
		case "confirm":
			// TODO: do some more or epochs math (round to miner PP, deal start buffer)

			afmt.Printf("-----\n")
			afmt.Printf("Proposing from %s\n", a)
			afmt.Printf("\tBalance: %s\n", types.FIL(fromBal))
			afmt.Printf("\n")
			afmt.Printf("Piece size: %s (Payload size: %s)\n", units.BytesSize(float64(ds.PieceSize)), units.BytesSize(float64(ds.PayloadSize)))
			afmt.Printf("Duration: %s\n", dur)

			pricePerGib := big.Zero()
			for _, a := range ask {
				p := a.Price
				if verified {
					p = a.VerifiedPrice
				}
				pricePerGib = big.Add(pricePerGib, p)
				epochPrice := types.BigDiv(types.BigMul(p, types.NewInt(uint64(ds.PieceSize))), gib)
				epochPrices = append(epochPrices, epochPrice)

				mpow, err := fapi.StateMinerPower(ctx, a.Miner, types.EmptyTSK)
				if err != nil {
					return fmt.Errorf("getting power (%s): %w", a.Miner, err)
				}

				if len(ask) > 1 {
					totalPrice := types.BigMul(epochPrice, types.NewInt(uint64(epochs)))
					afmt.Printf("Miner %s (Power:%s) price: ~%s (%s per epoch)\n", color.YellowString(a.Miner.String()), color.GreenString(types.SizeStr(mpow.MinerPower.QualityAdjPower)), color.BlueString(types.FIL(totalPrice).String()), types.FIL(epochPrice))
				}
			}

			// TODO: price is based on PaddedPieceSize, right?
			epochPrice := types.BigDiv(types.BigMul(pricePerGib, types.NewInt(uint64(ds.PieceSize))), gib)
			totalPrice := types.BigMul(epochPrice, types.NewInt(uint64(epochs)))

			afmt.Printf("Total price: ~%s (%s per epoch)\n", color.CyanString(types.FIL(totalPrice).String()), types.FIL(epochPrice))
			afmt.Printf("Verified: %v\n", verified)

			state = "accept"
		case "accept":
			afmt.Print("\nAccept (yes/no): ")

			_yn, _, err := rl.ReadLine()
			yn := string(_yn)
			if err != nil {
				return err
			}

			if yn == "no" {
				return nil
			}

			if yn != "yes" {
				afmt.Println("Type in full 'yes' or 'no'")
				continue
			}

			state = "execute"
		case "execute":
			color.Blue(".. executing\n")

			for i, maddr := range maddrs {
				proposal, err := api.ClientStartDeal(ctx, &client.DealParams{
					Data: &storagemarket.DataRef{
						TransferType: storagemarket.TTGraphsync,
						Root:         data,

						PieceCid:  &ds.PieceCID,
						PieceSize: ds.PieceSize.Unpadded(),
					},
					Wallet:            a,
					Miner:             maddr,
					EpochPrice:        epochPrices[i],
					MinBlocksDuration: uint64(epochs),
					DealStartEpoch:    abi.ChainEpoch(cctx.Int64("start-epoch")),
					FastRetrieval:     cctx.Bool("fast-retrieval"),
					VerifiedDeal:      verified,
				})
				if err != nil {
					return err
				}

				encoder, err := cli2.GetCidEncoder(cctx)
				if err != nil {
					return err
				}

				afmt.Printf("Deal (%s) CID: %s\n", maddr, color.GreenString(encoder.Encode(*proposal)))
			}

			return nil
		default:
			return fmt.Errorf("unknown state: %s", state)
		}
	}
}

var storageDealsListCmd = &cli.Command{
	Name:  "list",
	Usage: "List storage market deals",
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
		},
		&cli.BoolFlag{
			Name:  "watch",
			Usage: "watch deal updates in real-time, rather than a one time list",
		},
		&cli.BoolFlag{
			Name:  "offline",
			Usage: "only print offline deals",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.IsSet("color") {
			color.NoColor = !cctx.Bool("color")
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		verbose := cctx.Bool("verbose")
		watch := cctx.Bool("watch")
		showFailed := cctx.Bool("show-failed")

		var localDeals []client.DealInfo
		if cctx.Bool("offline") {
			localDeals, err = api.ClientListOfflineDeals(ctx)
			if err != nil {
				return err
			}
		} else {
			localDeals, err = api.ClientListDeals(ctx)
			if err != nil {
				return err
			}

			if watch {
				updates, err := api.ClientGetDealUpdates(ctx)
				if err != nil {
					return err
				}

				for {
					tm.Clear()
					tm.MoveCursor(1, 1)

					err = outputClientStorageDeals(ctx, tm.Screen, fapi, localDeals, verbose, showFailed)
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
							if existing.ProposalCid.Equals(updated.ProposalCid) {
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
		}

		return outputClientStorageDeals(ctx, cctx.App.Writer, fapi, localDeals, verbose, showFailed)
	},
}

func ellipsis(s string, length int) string {
	if length > 0 && len(s) > length {
		return "..." + s[len(s)-length:]
	}
	return s
}

func dealStateString(state storagemarket.StorageDealStatus) string {
	s := storagemarket.DealStates[state]
	switch state {
	case storagemarket.StorageDealError, storagemarket.StorageDealExpired:
		return color.RedString(s)
	case storagemarket.StorageDealActive:
		return color.GreenString(s)
	default:
		return s
	}
}

type deal struct {
	LocalDeal        client.DealInfo
	OnChainDealState market.DealState
}

func dealFromDealInfo(ctx context.Context, full v1api.FullNode, head *types.TipSet, v client.DealInfo) deal {
	if v.DealID == 0 {
		return deal{
			LocalDeal:        v,
			OnChainDealState: market.EmptyDealState(),
		}
	}

	onChain, err := full.StateMarketStorageDeal(ctx, v.DealID, head.Key())
	if err != nil {
		return deal{LocalDeal: v}
	}

	return deal{
		LocalDeal:        v,
		OnChainDealState: onChain.State.Iface(),
	}
}

func outputClientStorageDeals(ctx context.Context, out io.Writer, full v1api.FullNode, localDeals []client.DealInfo, verbose bool, showFailed bool) error {
	sort.Slice(localDeals, func(i, j int) bool {
		return localDeals[i].CreationTime.Before(localDeals[j].CreationTime)
	})

	head, err := full.ChainHead(ctx)
	if err != nil {
		return err
	}

	var deals []deal
	for _, localDeal := range localDeals {
		if showFailed || localDeal.State != storagemarket.StorageDealError {
			deals = append(deals, dealFromDealInfo(ctx, full, head, localDeal))
		}
	}

	if verbose {
		w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
		fmt.Fprintf(w, "Created\tDealCid\tDealId\tProvider\tState\tOn Chain?\tSlashed?\tPieceCID\tDataCID\tSize\tPrice\tDuration\tTransferChannelID\tTransferStatus\tVerified\tMessage\n")
		for _, d := range deals {
			onChain := "N"
			if d.OnChainDealState.SectorStartEpoch() != -1 {
				onChain = fmt.Sprintf("Y (epoch %d)", d.OnChainDealState.SectorStartEpoch())
			}

			slashed := "N"
			if d.OnChainDealState.SlashEpoch() != -1 {
				slashed = fmt.Sprintf("Y (epoch %d)", d.OnChainDealState.SlashEpoch())
			}

			price := types.FIL(types.BigMul(d.LocalDeal.PricePerEpoch, types.NewInt(d.LocalDeal.Duration)))
			transferChannelID := ""
			if d.LocalDeal.TransferChannelID != nil {
				transferChannelID = d.LocalDeal.TransferChannelID.String()
			}
			transferStatus := ""
			if d.LocalDeal.DataTransfer != nil {
				transferStatus = datatransfer.Statuses[d.LocalDeal.DataTransfer.Status]
				// TODO: Include the transferred percentage once this bug is fixed:
				// https://github.com/ipfs/go-graphsync/issues/126
				//fmt.Printf("transferred: %d / size: %d\n", d.LocalDeal.DataTransfer.Transferred, d.LocalDeal.Size)
				//if d.LocalDeal.Size > 0 {
				//	pct := (100 * d.LocalDeal.DataTransfer.Transferred) / d.LocalDeal.Size
				//	transferPct = fmt.Sprintf("%d%%", pct)
				//}
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%v\t%s\n",
				d.LocalDeal.CreationTime.Format(time.Stamp),
				d.LocalDeal.ProposalCid,
				d.LocalDeal.DealID,
				d.LocalDeal.Provider,
				dealStateString(d.LocalDeal.State),
				onChain,
				slashed,
				d.LocalDeal.PieceCID,
				d.LocalDeal.DataRef.Root,
				types.SizeStr(types.NewInt(d.LocalDeal.Size)),
				price,
				d.LocalDeal.Duration,
				transferChannelID,
				transferStatus,
				d.LocalDeal.Verified,
				d.LocalDeal.Message)
		}
		return w.Flush()
	}

	w := tablewriter.New(tablewriter.Col("DealCid"),
		tablewriter.Col("DealId"),
		tablewriter.Col("Provider"),
		tablewriter.Col("State"),
		tablewriter.Col("On Chain?"),
		tablewriter.Col("Slashed?"),
		tablewriter.Col("PieceCID"),
		tablewriter.Col("DataCID"),
		tablewriter.Col("Size"),
		tablewriter.Col("Price"),
		tablewriter.Col("Duration"),
		tablewriter.Col("Verified"),
		tablewriter.NewLineCol("Message"))

	for _, d := range deals {
		propcid := ellipsis(d.LocalDeal.ProposalCid.String(), 8)
		datacid := ellipsis(d.LocalDeal.DataRef.Root.String(), 8)

		onChain := "N"
		if d.OnChainDealState.SectorStartEpoch() != -1 {
			onChain = fmt.Sprintf("Y (epoch %d)", d.OnChainDealState.SectorStartEpoch())
		}

		slashed := "N"
		if d.OnChainDealState.SlashEpoch() != -1 {
			slashed = fmt.Sprintf("Y (epoch %d)", d.OnChainDealState.SlashEpoch())
		}

		piece := ellipsis(d.LocalDeal.PieceCID.String(), 8)

		price := types.FIL(types.BigMul(d.LocalDeal.PricePerEpoch, types.NewInt(d.LocalDeal.Duration)))

		w.Write(map[string]interface{}{
			"DealCid":   propcid,
			"DealId":    d.LocalDeal.DealID,
			"Provider":  d.LocalDeal.Provider,
			"State":     dealStateString(d.LocalDeal.State),
			"On Chain?": onChain,
			"Slashed?":  slashed,
			"PieceCID":  piece,
			"DataCID":   datacid,
			"Size":      types.SizeStr(types.NewInt(d.LocalDeal.Size)),
			"Price":     price,
			"Verified":  d.LocalDeal.Verified,
			"Duration":  d.LocalDeal.Duration,
			"Message":   d.LocalDeal.Message,
		})
	}

	return w.Flush(out)
}

var storageDealsStatsCmd = &cli.Command{
	Name:  "stats",
	Usage: "Print statistics about local storage deals",
	Flags: []cli.Flag{
		&cli.DurationFlag{
			Name: "newer-than",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		localDeals, err := api.ClientListDeals(ctx)
		if err != nil {
			return err
		}

		var totalSize uint64
		byState := map[storagemarket.StorageDealStatus][]uint64{}
		for _, deal := range localDeals {
			if cctx.IsSet("newer-than") {
				if time.Since(deal.CreationTime) > cctx.Duration("newer-than") {
					continue
				}
			}

			totalSize += deal.Size
			byState[deal.State] = append(byState[deal.State], deal.Size)
		}

		fmt.Printf("Total: %d deals, %s\n", len(localDeals), types.SizeStr(types.NewInt(totalSize)))

		type stateStat struct {
			state storagemarket.StorageDealStatus
			count int
			bytes uint64
		}

		stateStats := make([]stateStat, 0, len(byState))
		for state, deals := range byState {
			if state == storagemarket.StorageDealActive {
				state = math.MaxUint64 // for sort
			}

			st := stateStat{
				state: state,
				count: len(deals),
			}
			for _, b := range deals {
				st.bytes += b
			}

			stateStats = append(stateStats, st)
		}

		sort.Slice(stateStats, func(i, j int) bool {
			return int64(stateStats[i].state) < int64(stateStats[j].state)
		})

		for _, st := range stateStats {
			if st.state == math.MaxUint64 {
				st.state = storagemarket.StorageDealActive
			}
			fmt.Printf("%s: %d deals, %s\n", storagemarket.DealStates[st.state], st.count, types.SizeStr(types.NewInt(st.bytes)))
		}

		return nil
	},
}

var storageDealsGetCmd = &cli.Command{
	Name:  "get",
	Usage: "Print detailed deal information",
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return cli.ShowCommandHelp(cctx, cctx.Command.Name)
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		propcid, err := cid.Decode(cctx.Args().First())
		if err != nil {
			return err
		}

		di, err := api.ClientGetDealInfo(ctx, propcid)
		if err != nil {
			return err
		}

		out := map[string]interface{}{
			"DealInfo: ": di,
		}

		if di.DealID != 0 {
			onChain, err := fapi.StateMarketStorageDeal(ctx, di.DealID, types.EmptyTSK)
			if err != nil {
				return err
			}

			out["OnChain"] = onChain
		}

		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	},
}

func renderDeal(di *client.DealInfo) {
	color.Blue("Deal ID:      %d\n", int(di.DealID))
	color.Blue("Proposal CID: %s\n\n", di.ProposalCid.String())

	if di.DealStages == nil {
		color.Yellow("Deal was made with an older version of droplet did not collect detailed information about its stages")
		return
	}

	for _, stg := range di.DealStages.Stages {
		msg := fmt.Sprintf("%s %s: %s (expected duration: %s)", color.BlueString("Stage:"), color.BlueString(strings.TrimPrefix(stg.Name, "StorageDeal")), stg.Description, color.GreenString(stg.ExpectedDuration))
		if stg.UpdatedTime.Time().IsZero() {
			msg = color.YellowString(msg)
		}
		fmt.Println(msg)

		for _, l := range stg.Logs {
			fmt.Printf("  %s %s\n", color.YellowString(l.UpdatedTime.Time().UTC().Round(time.Second).Format(time.Stamp)), l.Log)
		}

		if stg.Name == "StorageDealStartDataTransfer" {
			for _, dtStg := range di.DataTransfer.Stages.Stages {
				fmt.Printf("        %s %s %s\n", color.YellowString(dtStg.CreatedTime.Time().UTC().Round(time.Second).Format(time.Stamp)), color.BlueString("Data transfer stage:"), color.BlueString(dtStg.Name))
				for _, l := range dtStg.Logs {
					fmt.Printf("              %s %s\n", color.YellowString(l.UpdatedTime.Time().UTC().Round(time.Second).Format(time.Stamp)), l.Log)
				}
			}
		}
	}
}

func inspectDealCmd(ctx context.Context, api clientapi.IMarketClient, proposalCid string, dealId int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	deals, err := api.ClientListDeals(ctx)
	if err != nil {
		return err
	}

	var di *client.DealInfo
	for i, cdi := range deals {
		if proposalCid != "" && cdi.ProposalCid.String() == proposalCid {
			di = &deals[i]
			break
		}

		if dealId != 0 && int(cdi.DealID) == dealId {
			di = &deals[i]
			break
		}
	}

	if di == nil {
		if proposalCid != "" {
			return fmt.Errorf("cannot find deal with proposal cid: %s", proposalCid)
		}
		if dealId != 0 {
			return fmt.Errorf("cannot find deal with deal id: %v", dealId)
		}
		return errors.New("you must specify proposal cid or deal id in order to inspect a deal")
	}

	// populate DealInfo.DealStages and DataTransfer.Stages
	di, err = api.ClientGetDealInfo(ctx, di.ProposalCid)
	if err != nil {
		return fmt.Errorf("cannot get deal info for proposal cid: %v", di.ProposalCid)
	}

	renderDeal(di)

	return nil
}

var storageDealsInspectCmd = &cli.Command{
	Name:  "inspect",
	Usage: "Inspect detailed information about deal's lifecycle and the various stages it goes through",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name: "deal-id",
		},
		&cli.StringFlag{
			Name: "proposal-cid",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := cli2.ReqContext(cctx)
		return inspectDealCmd(ctx, api, cctx.String("proposal-cid"), cctx.Int("deal-id"))
	},
}

type params struct {
	firstArg      string // may data cid or car dir
	from          address.Address
	miner         []address.Address
	price         types.FIL
	dur           int64
	provCol       big.Int
	statelessDeal bool
	isVerified    bool
	dcap          uint64
}

func dealParamsFromContext(cctx *cli.Context, api clientapi.IMarketClient, fapi v1api.FullNode, isBatch bool) (*params, error) {
	var start int
	var addrs []string
	var miners []address.Address
	if !isBatch {
		// [data, miner, price, dur]
		start = 2
		addrs = []string{cctx.Args().Get(1)}
	} else {
		// [price, dur]
		addrs = cctx.StringSlice("miner")
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("must pass miner")
	}
	for _, addrStr := range addrs {
		miner, err := address.NewFromString(addrStr)
		if err != nil {
			return nil, err
		}
		miners = append(miners, miner)
	}

	price, err := types.ParseFIL(cctx.Args().Get(start))
	if err != nil {
		return nil, err
	}
	dur, err := strconv.ParseInt(cctx.Args().Get(start+1), 10, 32)
	if err != nil {
		return nil, err
	}

	var provCol big.Int
	if pcs := cctx.String("provider-collateral"); pcs != "" {
		pc, err := big.FromString(pcs)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provider-collateral: %w", err)
		}
		provCol = pc
	}

	if abi.ChainEpoch(dur) < MinDealDuration {
		return nil, fmt.Errorf("minimum deal duration is %d blocks", MinDealDuration)
	}
	if abi.ChainEpoch(dur) > MaxDealDuration {
		return nil, fmt.Errorf("maximum deal duration is %d blocks", MaxDealDuration)
	}

	a, err := cli2.AddressFromContextOrDefault(cctx, api)
	if err != nil {
		return nil, err
	}

	// Check if the address is a verified client
	dcap, err := fapi.StateVerifiedClientStatus(cctx.Context, a, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	isVerified := dcap != nil

	// If the user has explicitly set the --verified-deal flag
	if cctx.IsSet("verified-deal") {
		// If --verified-deal is true, but the address is not a verified
		// client, return an error
		verifiedDealParam := cctx.Bool("verified-deal")
		if verifiedDealParam && !isVerified {
			return nil, fmt.Errorf("address %s does not have verified client status", a)
		}

		// Override the default
		isVerified = verifiedDealParam
	}

	p := &params{
		firstArg:      cctx.Args().Get(0),
		miner:         miners,
		from:          a,
		price:         price,
		dur:           dur,
		provCol:       provCol,
		statelessDeal: cctx.Bool("manual-stateless-deal"),
		isVerified:    isVerified,
	}
	if dcap != nil {
		p.dcap = uint64(dcap.Int64())
	}

	return p, nil
}

var storageDelesBatchCmd = &cli.Command{
	Name:  "batch",
	Usage: "Batch storage deals with miners",
	Description: `Make deals with miners.
price is measured in FIL/Epoch. Miners usually don't accept a bid
lower than their advertised ask (which is in FIL/GiB/Epoch). You can check a miners listed price
with './droplet-clientstorage asks query <miner address>'.
duration is how long the miner should store the data for, in blocks.
The minimum value is 518400 (6 months).`,
	ArgsUsage: "[price duration]",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "miner",
			Usage: "The address of the miner you wish to make a deal with, eg. --miner t010001 --miner t010002",
		},
		&cli.StringFlag{
			Name:     "manifest",
			Usage:    "Path to the manifest file",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "from",
			Usage: "specify address to fund the deal with",
		},
		&cli.Int64Flag{
			Name:  "start-epoch",
			Usage: "specify the epoch that the deal should start at",
			Value: -1,
		},
		&cli.BoolFlag{
			Name:  "fast-retrieval",
			Usage: "indicates that data should be available for fast retrieval",
			Value: true,
		},
		&cli.BoolFlag{
			Name:        "verified-deal",
			Usage:       "indicate that the deal counts towards verified client total",
			DefaultText: "true if client is verified, false otherwise",
		},
		&cli.StringFlag{
			Name:  "provider-collateral",
			Usage: "specify the requested provider collateral the miner should put up",
		},
		&cli.IntFlag{
			Name:  "start-index",
			Usage: "Starting from the nth deal",
		},
		&cli.IntFlag{
			Name:  "end-index",
			Usage: "At the end of the nth deal",
		},
		&cli.BoolFlag{
			Name:  "filter",
			Usage: "f+ requirements for LDN",
		},
		&cli2.CidBaseFlag,
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return fmt.Errorf("must pass two arguments")
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := cli2.ReqContext(cctx)
		p, err := dealParamsFromContext(cctx, api, fapi, true)
		if err != nil {
			return err
		}

		transferType := storagemarket.TTManual
		if p.price.Int64() != 0 {
			return fmt.Errorf("you must provide a 'price' of 0")
		}

		manifests, err := loadManifest(cctx.String("manifest"))
		if err != nil {
			return fmt.Errorf("load manifest error: %v", err)
		}

		var params []*client.DealParams
		var selector *selector
		startIdx := cctx.Int("start-index")
		endIdx := cctx.Int("end-index")
		currDatacap := p.dcap
		minerDeal := make(map[address.Address]int)

		if p.isVerified {
			dd, err := api.ClientGetVerifiedDealDistribution(ctx, p.miner, p.from)
			if err != nil {
				return err
			}
			selector = newSelector(dd, p.from, p.miner)
		}

		for i, m := range manifests {
			if i < startIdx {
				continue
			}
			if endIdx > 0 && i >= endIdx {
				break
			}

			dataRef := &storagemarket.DataRef{
				TransferType: transferType,
				Root:         m.payloadCID,
				PieceCid:     &m.pieceCID,
				PieceSize:    m.pieceSize,
				RawBlockSize: m.payloadSize,
			}

			miner := p.miner[i%len(p.miner)]
			if p.isVerified {
				paddedPiecedSize := uint64(m.pieceSize.Padded())
				if currDatacap < paddedPiecedSize {
					fmt.Printf("datacap %d less than piece size %d\n", currDatacap, paddedPiecedSize)
					break
				}

				if cctx.IsSet("filter") {
					miner = selector.selectMiner(m.pieceCID, paddedPiecedSize)
					if miner.Empty() {
						selector.printError()
						break
					}
				}
				currDatacap -= paddedPiecedSize
			}
			params = append(params, fillDealParams(cctx, p, dataRef, miner))
			minerDeal[miner]++
		}
		fmt.Printf("has %d deals need to publish", len(params))
		if len(params) == 0 {
			fmt.Println()
			return nil
		}
		for miner, count := range minerDeal {
			fmt.Printf(", %s: %d", miner, count)
		}
		fmt.Println()

		res, err := api.ClientBatchDeal(ctx, params)
		if err != nil {
			return err
		}

		for i, r := range res.Results {
			root := params[i].Data.Root.String()
			if len(r.Message) == 0 {
				fmt.Printf("create deal success, proposal cid: %v\n", r.ProposalCID)
			} else {
				fmt.Printf("create deal failed, playload cid: %v, error: %v\n", root, r.Message)
			}
		}

		return nil
	},
}

var verifiedDealStatsCmd = &cli.Command{
	Name:  "verified-deal-stat",
	Usage: "Print the distribution of verified deals",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "provider",
			Usage: "provider address",
		},
		&cli.StringFlag{
			Name:  "client",
			Usage: "datacap address",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := cli2.ReqContext(cctx)
		var provider, clientAddr address.Address
		if cctx.IsSet("provider") {
			provider, err = address.NewFromString(cctx.String("provider"))
			if err != nil {
				return err
			}
		} else if cctx.IsSet("client") {
			clientAddr, err = address.NewFromString(cctx.String("client"))
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("must pass --provider of --client")
		}

		dd, err := api.ClientGetVerifiedDealDistribution(ctx, []address.Address{provider}, clientAddr)
		if err != nil {
			return err
		}

		newProviderWriter := func() *tablewriter.TableWriter {
			return tablewriter.New(
				tablewriter.Col("Provider"),
				tablewriter.Col("Total"),
				tablewriter.Col("Percentage"),
				tablewriter.Col("Uniq"),
				tablewriter.Col("DuplicationPercentage"),
			)
		}

		sizeStr := func(v uint64) string {
			return fmt.Sprintf("%s (%d B)", types.SizeStr(types.NewInt(v)), v)
		}

		writeProviderDistribution := func(writer *tablewriter.TableWriter, pd *client.ProviderDistribution, percentage float64) {
			rows := map[string]interface{}{
				"Provider":              pd.Provider,
				"Total":                 sizeStr(pd.Total),
				"Uniq":                  sizeStr(pd.Uniq),
				"DuplicationPercentage": fmt.Sprintf("%.2f%s", 100*pd.DuplicationPercentage, "%"),
			}
			if percentage != 0 {
				rows["Percentage"] = fmt.Sprintf("%.2f%s", 100*percentage, "%")
			}
			writer.Write(rows)
		}

		writeReplicasDistribution := func(buf *bytes.Buffer, rd *client.ReplicaDistribution) {
			writer := newProviderWriter()
			for _, pd := range rd.ReplicasDistribution {
				writeProviderDistribution(writer, pd, rd.ReplicasPercentage[pd.Provider.String()])
			}
			fmt.Fprintf(buf, "Client: %s\n", rd.Client)
			fmt.Fprintf(buf, "Total:  %s\n", sizeStr(rd.Total))
			fmt.Fprintf(buf, "Uniq:   %s\n", sizeStr(rd.Uniq))
			fmt.Fprintf(buf, "DuplicationPercentage: %.2f%s\n", rd.DuplicationPercentage*100, "%")
			_ = writer.Flush(buf)
			buf.WriteString("\n")
		}

		for _, pd := range dd.ProvidersDistribution {
			if pd.Provider == provider {
				writer := newProviderWriter()
				writeProviderDistribution(writer, pd, 0)

				return writer.Flush(os.Stdout)
			}
		}

		buf := new(bytes.Buffer)
		for _, rd := range dd.ReplicasDistribution {
			if rd.Client == clientAddr {
				writeReplicasDistribution(buf, rd)
				break
			}
		}
		fmt.Fprint(os.Stdout, buf.String())

		return nil
	},
}

var storageDealsExportCmd = &cli.Command{
	Name:  "export",
	Usage: "Export deal proposal cid and piece cid when the deal status is StorageDealWaitingForData",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "output",
			Usage: "output result to file",
		},
		&cli.IntFlag{
			Name:  "count",
			Usage: "number of exported deals",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := cli2.ReqContext(cctx)

		var deals []*client.DealInfo
		res, err := api.ClientListOfflineDeals(ctx)
		if err != nil {
			return err
		}
		for i := range res {
			if res[i].State == storagemarket.StorageDealWaitingForData {
				deals = append(deals, &res[i])
			}
		}
		if len(deals) == 0 {
			fmt.Println("no deals need export")
			return nil
		}

		if cctx.IsSet("output") {
			count := cctx.Int("count")
			buf := &bytes.Buffer{}
			buf.WriteString("proposalCID,pieceCID\n")
			for i, deal := range deals {
				if count > 0 && i >= count {
					continue
				}
				buf.WriteString(fmt.Sprintf("%s,%s\n", deal.ProposalCid.String(), deal.PieceCID.String()))
			}

			return os.WriteFile(cctx.String("output"), buf.Bytes(), 0o755)
		}
		writer := tablewriter.New(tablewriter.Col("ProposalCID"), tablewriter.Col("PieceCID"))
		for _, deal := range deals {
			writer.Write(map[string]interface{}{
				"ProposalCID": deal.ProposalCid.String(),
				"PieceCID":    deal.PieceCID.String(),
			})
		}

		return writer.Flush(os.Stdout)
	},
}
