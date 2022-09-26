package cli

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
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
		var withMinPower []address.Address
		minerToMinerPower := make(map[address.Address]power.Claim)
		minerToTotalPower := make(map[address.Address]power.Claim)

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

				if power.HasMinPower {
					lk.Lock()
					withMinPower = append(withMinPower, miner)
					minerToMinerPower[miner] = power.MinerPower
					minerToTotalPower[miner] = power.TotalPower
					lk.Unlock()
				}
			}(miner)
		}

		wg.Wait()

		log.Println("Total SPs with minimum power: ", len(withMinPower))

		var venusNodes int

		RawBytePower := big.NewInt(0)
		QualityAdjPower := big.NewInt(0)

		host, err := libp2p.New(libp2p.NoListenAddrs)
		if err != nil {
			return err
		}

		for _, maddr := range withMinPower {
			select {
			case <-ctx.Done():
				return nil
			default:
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

				if strings.Contains(userAgent, "venus") {

					log.Println("Provider %s is running venus" + maddr.String())
					log.Println("venus provider ", maddr.String(), "raw power:", minerToMinerPower[maddr].RawBytePower)
					log.Println("venus provider ", maddr.String(), "quality adj power:", minerToMinerPower[maddr].QualityAdjPower)

					venusNodes++
					QualityAdjPower = big.Add(QualityAdjPower, minerToMinerPower[maddr].QualityAdjPower)
					RawBytePower = big.Add(RawBytePower, minerToMinerPower[maddr].RawBytePower)
				}

				return nil
			}()
			if err != nil {
				log.Println("warn: ", err)
				continue
			}
		}

		fmt.Println()
		fmt.Println("Total venus nodes:", venusNodes)
		fmt.Println("Total venus raw power:", types.DeciStr(RawBytePower))
		fmt.Println("Total venus quality adj power:", types.DeciStr(QualityAdjPower))
		fmt.Println("Total SPs with minimum power: ", len(withMinPower))

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

		var totalDealSize = big.Zero()
		var count = 0

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
