package cli

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

var indexProvCmd = &cli.Command{
	Name:  "index",
	Usage: "Manage the index provider on Boost",
	Subcommands: []*cli.Command{
		indexProvAnnounceAllCmd,
		indexProvListMultihashesCmd,
		indexProvAnnounceLatest,
		indexProvAnnounceLatestHttp,
		indexProvAnnounceDealRemovalAd,
		indexProvAnnounceDeal,
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
