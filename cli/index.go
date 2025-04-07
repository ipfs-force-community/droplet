package cli

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/ipni/go-libipni/find/client"
	"github.com/ipni/go-libipni/find/model"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

var IndexProvCmd = &cli.Command{
	Name:  "index",
	Usage: "Manage the index provider on Boost",
	Subcommands: []*cli.Command{
		indexProvAnnounceAllCmd,
		indexProvListMultihashesCmd,
		indexProvAnnounceLatest,
		indexProvAnnounceLatestHttp,
		indexProvAnnounceDealRemovalAd,
		indexProvAnnounceDeal,
		checkDealIndexCmd,
	},
}

var checkDealIndexCmd = &cli.Command{
	Name:  "check-deal-index",
	Usage: "Check if a deal is indexed by the index provider",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "miner",
			Usage: `specify miner address`,
		},
		&cli.StringFlag{
			Name:     "droplet-url",
			Usage:    "specify the url of the droplet node",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "droplet-token",
			Usage:    "specify the token of the droplet node",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "node-url",
			Usage:    "specify the url of the full node",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "node-token",
			Usage:    "specify the token of the full node",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "start",
			Usage: "check index from this time, eg. 2024-01-01, default is 1 year ago",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "verbose output",
		},
		&cli.StringFlag{
			Name:  "ipni-url",
			Value: "https://cid.contact",
		},
	},
	Action: func(cctx *cli.Context) error {
		token := cctx.String("droplet-token")
		url := cctx.String("droplet-url")
		nodeURL := cctx.String("node-url")
		nodeToken := cctx.String("node-token")

		ctx := ReqContext(cctx)
		nodeAPI, closer, err := DailDropletNode(ctx, url, token)
		if err != nil {
			return err
		}
		defer closer()

		full, fcloser, err := DailFullNode(ctx, nodeURL, nodeToken)
		if err != nil {
			return err
		}
		defer fcloser()

		start := time.Now().Add(-365 * 24 * time.Hour)
		if cctx.IsSet("start") {
			var err error
			start, err = time.Parse(time.DateOnly, cctx.String("start"))
			if err != nil {
				return fmt.Errorf("invalid start time(%s): %v", cctx.String("start"), err)
			}
			fmt.Println("check index from: ", start)
		}
		deals, _, err := getDeals(ctx, cctx, nodeAPI, start)
		if err != nil {
			return err
		}

		dealIndexs := make(map[string]*dealIndex)
		startUnix := start.Unix()
		verboose := cctx.Bool("verbose")

		ipniURL := cctx.String("ipni-url")
		cli, err := client.New(ipniURL)
		if err != nil {
			return err
		}

		peerIDs := make(map[string]string)
		getMinerPeerID := func(miner string) (string, error) {
			peerID, ok := peerIDs[miner]
			if ok {
				return peerID, nil
			}

			mAddr, err := address.NewFromString(miner)
			if err != nil {
				return "", fmt.Errorf("invalid miner address %s: %v", miner, err)
			}
			mi, err := full.StateMinerInfo(ctx, mAddr, types.EmptyTSK)
			if err != nil {
				return "", fmt.Errorf("get miner %s peer id failed: %v", miner, err)
			}
			if mi.PeerId == nil {
				return "", fmt.Errorf("miner %s peer id is nil", miner)
			}
			peerIDs[miner] = mi.PeerId.String()

			return mi.PeerId.String(), nil
		}

		resCache := make(map[string]*model.FindResponse)
		fillDealIndex := func(id, miner, payloadCID string) error {
			di, ok := dealIndexs[miner]
			if !ok {
				di = &dealIndex{}
				dealIndexs[miner] = di
			}
			di.dealCount++

			res, ok := resCache[payloadCID]
			if !ok {
				result, err := cli.FindByPayloadCID(ctx, payloadCID)
				if err != nil {
					return fmt.Errorf("get index by miner %s payload cid %s faield: %v", miner, payloadCID, err)
				}
				res = result
				resCache[payloadCID] = res
			}

			peerID, err := getMinerPeerID(miner)
			if err != nil {
				return err
			}
			for _, mrs := range res.MultihashResults {
				for _, pr := range mrs.ProviderResults {
					if pr.Provider == nil {
						continue
					}
					if pr.Provider.ID.String() == peerID {
						di.indexCount++
						return nil
					}
				}
			}
			if verboose {
				fmt.Printf("miner %s deal %s not indexed: %s\n", miner, id, payloadCID)
			}

			return nil
		}

		for _, deal := range deals {
			if deal.CreatedAt < uint64(startUnix) {
				continue
			}
			label, err := deal.Proposal.Label.ToString()
			if err != nil {
				fmt.Printf("deal %s label is not string: %v\n", deal.ProposalCid, err)
				continue
			}
			if err := fillDealIndex(deal.ProposalCid.String(), deal.Proposal.Provider.String(), label); err != nil {
				fmt.Println("fill deal index failed: ", err)
				break
			}
		}

		// todo: get payload cid from dDeals
		// for _, deal := range dDeals {
		// 	if deal.CreatedAt < uint64(startUnix) {
		// 		continue
		// 	}
		// }

		for miner, mi := range dealIndexs {
			fmt.Printf("miner: %s, deal count: %d, index count: %d\n", miner, mi.dealCount, mi.indexCount)
		}

		return nil
	},
}

var indexProvAnnounceAllCmd = &cli.Command{
	Name:  "announce-all",
	Usage: "Announce all active deals to indexers so they can download the indices",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "miner",
			Usage:    `specify miner address`,
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		minerAddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return err
		}

		return nodeAPI.IndexerAnnounceAllDeals(ctx, minerAddr)
	},
}

var indexProvListMultihashesCmd = &cli.Command{
	Name:      "list-multihashes",
	Usage:     "list-multihashes <proposal cid / deal UUID>",
	UsageText: "List multihashes for a deal by proposal cid or deal UUID",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return fmt.Errorf("must supply a proposal cid or deal UUID")
		}

		ctx := ReqContext(cctx)

		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must specify only one proposal CID / deal UUID")
		}

		id := cctx.Args().Get(0)

		var proposalCid cid.Cid
		var mhs []multihash.Multihash
		dealUuid, err := uuid.Parse(id)
		if err != nil {
			propCid, err := cid.Decode(id)
			if err != nil {
				return fmt.Errorf("could not parse '%s' as deal uuid or proposal cid", id)
			}
			proposalCid = propCid
		}

		if !proposalCid.Defined() {
			contextID, err := dealUuid.MarshalBinary()
			if err != nil {
				return fmt.Errorf("parsing UUID to bytes: %w", err)
			}
			mhs, err = nodeAPI.IndexerListMultihashes(ctx, contextID)
			if err != nil {
				return err
			}
			fmt.Printf("Found %d multihashes for deal with ID %s:\n", len(mhs), id)
			for _, mh := range mhs {
				fmt.Println("  " + mh.String())
			}

			return nil
		}

		mhs, err = nodeAPI.IndexerListMultihashes(ctx, proposalCid.Bytes())
		if err != nil {
			return err
		}

		fmt.Printf("Found %d multihashes for deal with ID %s:\n", len(mhs), id)
		for _, mh := range mhs {
			fmt.Println("  " + mh.String())
		}

		return nil
	},
}

var indexProvAnnounceLatest = &cli.Command{
	Name:  "announce-latest",
	Usage: "Re-publish the latest existing advertisement to pubsub",
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)

		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		c, err := nodeAPI.IndexerAnnounceLatest(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("Announced advertisement with cid %s\n", c)
		return nil
	},
}

var indexProvAnnounceLatestHttp = &cli.Command{
	Name:  "announce-latest-http",
	Usage: "Re-publish the latest existing advertisement to specific indexers over http",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:     "announce-url",
			Usage:    "The url(s) to announce to. If not specified, announces to the http urls in config",
			Required: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)

		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		c, err := nodeAPI.IndexerAnnounceLatestHttp(ctx, cctx.StringSlice("announce-url"))
		if err != nil {
			return err
		}

		fmt.Printf("Announced advertisement to indexers over http with cid %s\n", c)
		return nil
	},
}

var indexProvAnnounceDealRemovalAd = &cli.Command{
	Name:  "announce-remove-deal",
	Usage: "Published a removal ad for given deal UUID or Signed Proposal CID (legacy deals)",
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)

		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must specify only one proposal CID / deal UUID")
		}

		id := cctx.Args().Get(0)

		var contextID []byte
		dealUuid, err := uuid.Parse(id)
		if err != nil {
			propCid, err := cid.Decode(id)
			if err != nil {
				return fmt.Errorf("could not parse '%s' as deal uuid or proposal cid", id)
			}
			contextID = propCid.Bytes()
		} else {
			contextID, err = dealUuid.MarshalBinary()
			if err != nil {
				return fmt.Errorf("parsing UUID to bytes: %w", err)
			}
		}

		cid, err := nodeAPI.IndexerAnnounceDealRemoved(ctx, contextID)
		if err != nil {
			return fmt.Errorf("failed to send removal ad: %w", err)
		}
		fmt.Printf("Announced the removal Ad with cid %s\n", cid)

		return nil
	},
}

var indexProvAnnounceDeal = &cli.Command{
	Name:  "announce-deal",
	Usage: "Publish an ad for for given deal UUID or Signed Proposal CID (legacy deals)",
	Action: func(cctx *cli.Context) error {
		ctx := ReqContext(cctx)

		nodeAPI, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.Args().Len() != 1 {
			return fmt.Errorf("must specify only one deal UUID")
		}

		id := cctx.Args().Get(0)

		var contextID []byte
		dealUuid, err := uuid.Parse(id)
		if err != nil {
			propCid, err := cid.Decode(id)
			if err != nil {
				return fmt.Errorf("could not parse '%s' as deal uuid or proposal cid", id)
			}
			contextID = propCid.Bytes()
		} else {
			contextID, err = dealUuid.MarshalBinary()
			if err != nil {
				return fmt.Errorf("parsing UUID to bytes: %w", err)
			}
		}

		ad, err := nodeAPI.IndexerAnnounceDeal(ctx, contextID)
		if err != nil {
			return fmt.Errorf("announcing deal failed: %v", err)
		}
		if ad.Defined() {
			fmt.Printf("Announced the  deal with Ad cid %s\n", ad)
			return nil
		}
		fmt.Printf("deal already announced\n")
		return nil
	},
}
