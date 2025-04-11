package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	marketapi "github.com/filecoin-project/venus/venus-shared/api/market/v1"
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
			Name:     "miner",
			Usage:    `specify miner address`,
			Required: true,
		},
		&cli.StringFlag{
			Name:  "droplet-url",
			Usage: "specify the url of the droplet node",
		},
		&cli.StringFlag{
			Name:  "droplet-token",
			Usage: "specify the token of the droplet node",
		},
		&cli.StringFlag{
			Name:  "node-url",
			Usage: "specify the url of the full node",
		},
		&cli.StringFlag{
			Name:  "node-token",
			Usage: "specify the token of the full node",
		},
		&cli.StringFlag{
			Name:  "start",
			Usage: "check index from this time, eg. 2024-01-01, default is 180 days ago",
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
		&cli.BoolFlag{
			Name:  "try-announce",
			Usage: "try to announce the deal to indexer if not indexed",
			Value: true,
		},
		&cli.StringFlag{
			Name: "output",
		},
	},
	Action: func(cctx *cli.Context) error {
		token := cctx.String("droplet-token")
		url := cctx.String("droplet-url")
		nodeURL := cctx.String("node-url")
		nodeToken := cctx.String("node-token")
		tryAnnounce := cctx.Bool("try-announce")
		fmt.Println("droplet url: ", token)
		fmt.Println("droplet token: ", url)
		fmt.Println("node url: ", nodeURL)
		fmt.Println("node token: ", nodeToken)

		ctx := ReqContext(cctx)
		var nodeAPI marketapi.IMarket
		var closer, fcloser jsonrpc.ClientCloser
		var full v1.FullNode
		var err error
		if len(url) == 0 || len(token) == 0 {
			nodeAPI, closer, err = NewMarketNode(cctx)
			if err != nil {
				return err
			}
			defer closer()

			full, fcloser, err = NewFullNode(cctx, OldMarketRepoPath)
			if err != nil {
				return err
			}
			defer fcloser()
		} else {
			nodeAPI, closer, err = DailDropletNode(ctx, url, token)
			if err != nil {
				return err
			}
			defer closer()

			full, fcloser, err = DailFullNode(ctx, nodeURL, nodeToken)
			if err != nil {
				return err
			}
			defer fcloser()
		}

		start := time.Now().Add(-180 * 24 * time.Hour)
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

		payloadCIDPeerIDs := make(map[string]map[string]struct{}, 0)

		minerStr := cctx.String("miner")
		mf := fmt.Sprintf("%s_payload_cid_peer_ids.json", minerStr)
		if output := cctx.String("output"); len(output) > 0 {
			mf = output
		}
		fmt.Println("payload cid peer ids file: ", mf)

		d, err := os.ReadFile(mf)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("read payload cid peer ids file %s failed: %v", mf, err)
			}
			fmt.Printf("read payload cid peer ids file %s not exist\n", mf)
		} else {
			if err := json.Unmarshal(d, &payloadCIDPeerIDs); err != nil {
				fmt.Printf("unmarshal payload cid peer ids file %s failed: %v\n", mf, err)
				payloadCIDPeerIDs = make(map[string]map[string]struct{}, 0)
			}
			fmt.Printf("payload cid peer ids count: %d\n", len(payloadCIDPeerIDs))
		}

		savePayloadCIDPeerIDs := func(payloadCID string, res *model.FindResponse) {
			for _, mrs := range res.MultihashResults {
				for _, pr := range mrs.ProviderResults {
					if pr.Provider == nil {
						continue
					}
					_, ok := payloadCIDPeerIDs[payloadCID]
					if !ok {
						payloadCIDPeerIDs[payloadCID] = make(map[string]struct{})
					}
					payloadCIDPeerIDs[payloadCID][pr.Provider.ID.String()] = struct{}{}
				}
			}
		}

		defer func() {
			data, err := json.Marshal(payloadCIDPeerIDs)
			if err != nil {
				fmt.Printf("json marshal payload cid peer ids failed: %v\n", err)
			} else {
				if err := os.WriteFile(mf, data, 0644); err != nil {
					fmt.Printf("write payload cid peer ids to file failed: %v\n", err)
				} else {
					fmt.Printf("write payload cid peer ids to file success\n")
				}
			}
		}()

		fillDealIndex := func(miner, payloadCID string, propoCID cid.Cid) error {
			id := propoCID.String()
			di, ok := dealIndexs[miner]
			if !ok {
				di = &dealIndex{}
				dealIndexs[miner] = di
			}
			di.dealCount++

			_, ok = payloadCIDPeerIDs[payloadCID]
			if !ok {
				res, err := cli.FindByPayloadCID(ctx, payloadCID)
				if err != nil {
					return fmt.Errorf("get index by miner %s payload cid %s faield: %v", miner, payloadCID, err)
				}
				savePayloadCIDPeerIDs(payloadCID, res)
			}

			peerID, err := getMinerPeerID(miner)
			if err != nil {
				return err
			}

			_, ok = payloadCIDPeerIDs[payloadCID][peerID]
			if ok {
				di.indexCount++
				return nil
			}
			if verboose {
				fmt.Printf("miner %s deal %s not indexed, payload cid: %s\n", miner, id, payloadCID)
			}
			if tryAnnounce {
				fmt.Printf("try announce deal %s, %s\n", id, payloadCID)

				ctxID := propoCID.Bytes()
				ad, err := nodeAPI.IndexerAnnounceDealRemoved(ctx, ctxID)
				if err != nil {
					fmt.Printf("announce deal remove failed: %v\n", err)
				} else {
					fmt.Printf("announce deal remove success, cid: %s\n", ad)
				}

				ad, err = nodeAPI.IndexerAnnounceDeal(ctx, ctxID)
				if err != nil {
					fmt.Printf("announce deal failed: %v\n", err)
				} else {
					fmt.Printf("announce deal success, ad: %v\n", ad)
				}
			}

			return nil
		}

		sort.Slice(deals, func(i, j int) bool {
			return deals[i].CreatedAt > deals[j].CreatedAt
		})

		for _, deal := range deals {
			if deal.CreatedAt < uint64(startUnix) {
				continue
			}
			label, err := deal.Proposal.Label.ToString()
			if err != nil {
				fmt.Printf("deal %s label is not string: %v\n", deal.ProposalCid, err)
				continue
			}
			if err := fillDealIndex(deal.Proposal.Provider.String(), label, deal.ProposalCid); err != nil {
				fmt.Println("fill deal index failed: ", err)
				time.Sleep(time.Minute * 3)
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
