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
		StatsDealsCmd,
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

		api, acloser, err := NewFullNode(cctx, OldMarketRepoPath)
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

		wg.Add(len(minerInfos))
		for maddr, info := range minerInfos {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			go func(mAddr address.Address, info *minerInfo) {
				throttle <- struct{}{}
				defer func() {
					wg.Done()
					<-throttle
				}()

				if !info.Power.RawBytePower.GreaterThan(big.Zero()) {
					log.Println("Skipping SP with no power: ", mAddr)
					return
				}
				if !info.HasMinPower && !cctx.Bool("min-power") {
					log.Println("Skipping SP with no min power: ", mAddr)
					return
				}

				err := func() error {
					log.Println("Checking SP: ", mAddr)

					mInfo, err := api.StateMinerInfo(ctx, mAddr, types.EmptyTSK)
					if err != nil {
						return err
					}
					if mInfo.PeerId == nil {
						return fmt.Errorf("storage provider %s has no peer ID set on-chain", mAddr)
					}

					var mAddrs []multiaddr.Multiaddr
					for _, mma := range mInfo.Multiaddrs {
						ma, err := multiaddr.NewMultiaddrBytes(mma)
						if err != nil {
							return fmt.Errorf("storage provider %s had invalid multiaddrs in their info: %w", mAddr, err)
						}
						mAddrs = append(mAddrs, ma)
					}
					if len(mAddrs) == 0 {
						return fmt.Errorf("storage provider %s has no multiaddrs set on-chain", mAddr)
					}

					addrInfo := peer.AddrInfo{
						ID:    *mInfo.PeerId,
						Addrs: mAddrs,
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
						log.Printf("Provider %s is running venus", mAddr.String())
						log.Println("venus provider ", mAddr.String(), "raw power:", info.Power.RawBytePower)
						log.Println("venus provider ", mAddr.String(), "quality adj power:", info.Power.QualityAdjPower)

						venusNodes++
						QualityAdjPower = big.Add(QualityAdjPower, info.Power.QualityAdjPower)
						RawBytePower = big.Add(RawBytePower, info.Power.RawBytePower)
					}

					return nil
				}()
				if err != nil {
					log.Println("warn: ", err)
					return
				}

			}(maddr, info)

		}
		wg.Wait()

		if cctx.Bool("list") {
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "Miner\tAgent\tQualityAdjPower\tRawBytePower\tHasMinPower")
			for maddr, info := range minerInfos {
				if info.HasMinPower {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%t\n", maddr, info.Agent, info.Power.QualityAdjPower, info.Power.RawBytePower, info.HasMinPower)
				}
			}
			for maddr, info := range minerInfos {
				if !info.HasMinPower && info.Power.RawBytePower.GreaterThan(big.Zero()) && cctx.Bool("min-power") {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%t\n", maddr, info.Agent, info.Power.QualityAdjPower, info.Power.RawBytePower, info.HasMinPower)
				}
			}
			err := w.Flush()
			if err != nil {
				return err
			}
		} else if cctx.Bool("json") {
			minerInfosMarshallable := make(map[string]*minerInfo)
			for maddr, info := range minerInfos {
				if info.HasMinPower || !cctx.Bool("min-power") && info.Power.RawBytePower.GreaterThan(big.Zero()) {
					minerInfosMarshallable[maddr.String()] = info
				}
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
		return nil
	},
}

var StatsDealsCmd = &cli.Command{
	Name:        "deals",
	Description: "Statistics on active market deals",
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)
		api, closer, err := NewFullNode(cctx, OldMarketRepoPath)
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
