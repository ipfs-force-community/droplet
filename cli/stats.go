package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/urfave/cli/v2"
)

var StatsCmds = &cli.Command{
	Name:  "stats",
	Usage: "Stats about deals, sectors, and other things",
	Subcommands: []*cli.Command{
		StatsPowerCmd,
		StatsDealskCmd,
	},
}

var StatsPowerCmd = &cli.Command{
	Name:        "power",
	Description: "Statistics on how many SPs are running Venus",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Usage:   "verbose output",
			Aliases: []string{"v", "debug"},
		},
		&cli.BoolFlag{
			Name:  "list",
			Usage: "list all miners with minPower ",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "list all miners with minPower output as json",
		},
		&cli.BoolFlag{
			Name:  "min-power",
			Usage: "include miners without minPower",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)
		if cctx.Bool("verbose") {
			log.SetOutput(os.Stdout)
		} else {
			log.SetOutput(bytes.NewBuffer(nil))
		}

		api, acloser, err := NewFullNode(cctx)
		if err != nil {
			return fmt.Errorf("setting up venus node connection: %w", err)
		}
		defer acloser()

		miners, err := api.StateListMiners(ctx, types.EmptyTSK)
		if err != nil {
			return err
		}

		log.Println("Total SPs on chain: ", len(miners))

		var wg sync.WaitGroup
		wg.Add(len(miners))
		var lk sync.Mutex

		type minerInfo struct {
			Agent       string
			Power       power.Claim
			HasMinPower bool
		}

		minerInfos := make(map[address.Address]*minerInfo)
		minerWithMinPowerCount := 0

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
					panic(err)
				}

				info := &minerInfo{
					HasMinPower: power.HasMinPower,
					Power:       power.MinerPower,
					Agent:       "unknown",
				}

				lk.Lock()
				minerInfos[miner] = info
				lk.Unlock()
			}(miner)
		}

		wg.Wait()

		var venusNodes int

		RawBytePower := big.NewInt(0)
		QualityAdjPower := big.NewInt(0)

		host, err := libp2p.New(libp2p.NoListenAddrs)
		if err != nil {
			return err
		}

		for maddr, info := range minerInfos {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			if !info.Power.RawBytePower.GreaterThan(big.Zero()) {
				log.Println("Skipping SP with no power: ", maddr)
				continue
			}
			if !info.HasMinPower && !cctx.Bool("min-power") {
				log.Println("Skipping SP with no min power: ", maddr)
				continue
			}

			err := func() error {
				log.Println("Checking SP: ", maddr)

				minfo, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
				if err != nil {
					return err
				}
				if minfo.PeerId == nil {
					return fmt.Errorf("storage provider %s has no peer ID set on-chain", maddr)
				}

				var maddrs []multiaddr.Multiaddr
				for _, mma := range minfo.Multiaddrs {
					ma, err := multiaddr.NewMultiaddrBytes(mma)
					if err != nil {
						return fmt.Errorf("storage provider %s had invalid multiaddrs in their info: %w", maddr, err)
					}
					maddrs = append(maddrs, ma)
				}
				if len(maddrs) == 0 {
					return fmt.Errorf("storage provider %s has no multiaddrs set on-chain", maddr)
				}

				addrInfo := peer.AddrInfo{
					ID:    *minfo.PeerId,
					Addrs: maddrs,
				}

				if err := host.Connect(ctx, addrInfo); err != nil {
					return fmt.Errorf("connecting to peer %s: %w", addrInfo.ID, err)
				}

				userAgentI, err := host.Peerstore().Get(addrInfo.ID, "AgentVersion")
				if err != nil {
					return fmt.Errorf("getting user agent for peer %s: %w", addrInfo.ID, err)
				}

				userAgent, ok := userAgentI.(string)
				if !ok {
					return fmt.Errorf("user agent for peer %s was not a string", addrInfo.ID)
				}
				log.Println("User agent: ", userAgent)

				info.Agent = userAgent

				if strings.Contains(userAgent, "venus") {
					log.Println("Provider %s is running venus" + maddr.String())
					log.Println("venus provider ", maddr.String(), "raw power:", info.Power.RawBytePower)
					log.Println("venus provider ", maddr.String(), "quality adj power:", info.Power.QualityAdjPower)

					venusNodes++
					QualityAdjPower = big.Add(QualityAdjPower, info.Power.QualityAdjPower)
					RawBytePower = big.Add(RawBytePower, info.Power.RawBytePower)
				}

				return nil
			}()
			if err != nil {
				log.Println("warn: ", err)
				continue
			}
		}

		fmt.Println(minerInfos)

		if cctx.Bool("list") {
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "Miner\tAgent\tQualityAdjPower\tRawBytePower\tHasMinPower")
			for maddr, info := range minerInfos {
				if info.HasMinPower {
					fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%t\n", maddr, info.Agent, info.Power.QualityAdjPower, info.Power.RawBytePower, info.HasMinPower)
				}
			}
			for maddr, info := range minerInfos {
				if !info.HasMinPower {
					fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%t\n", maddr, info.Agent, info.Power.QualityAdjPower, info.Power.RawBytePower, info.HasMinPower)
				}
			}
			w.Flush()
		} else if cctx.Bool("json") {
			minerInfosMarshallable := make(map[string]*minerInfo)
			for maddr, info := range minerInfos {
				minerInfosMarshallable[maddr.String()] = info
			}

			out, err := json.MarshalIndent(minerInfosMarshallable, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
		} else {
			fmt.Println()
			fmt.Println("Total venus nodes:", venusNodes)
			fmt.Println("Total venus raw power:", types.DeciStr(RawBytePower))
			fmt.Println("Total venus quality adj power:", types.DeciStr(QualityAdjPower))
			fmt.Println("Total SPs with minimum power: ", minerWithMinPowerCount)
		}
		os.Stdout.Sync()

		return nil
	},
}

var StatsDealskCmd = &cli.Command{
	Name:        "deals",
	Description: "Statistics on active market deals",
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)
		api, closer, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		deals, err := api.StateMarketDeals(ctx, types.EmptyTSK)
		if err != nil {
			return err
		}

		totalDealSize := big.Zero()
		count := 0

		for _, deal := range deals {
			state := deal.State
			if state.SectorStartEpoch > -1 && state.SlashEpoch == -1 {
				dealSize := big.NewIntUnsigned(uint64(deal.Proposal.PieceSize))
				totalDealSize = big.Add(totalDealSize, dealSize)
				count++
			}
		}

		fmt.Println("Total deals: ", count)
		fmt.Println("Total deal size: ", types.SizeStr(totalDealSize))

		return nil
	},
}
