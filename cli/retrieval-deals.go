package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	mtypes "github.com/ipfs-force-community/droplet/v2/types"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
)

var RetrievalCmds = &cli.Command{
	Name:  "retrieval",
	Usage: "Manage retrieval deals and related configuration",
	Subcommands: []*cli.Command{
		retrievalDealsCmds,
		retirevalAsksCmds,
		retrievalDealSelectionCmds,
		queryProtocols,
	},
}

var retrievalDealsCmds = &cli.Command{
	Name:  "deal",
	Usage: "Manage retrieval deals and related configuration",
	Subcommands: []*cli.Command{
		retrievalDealsListCmd,
		getRetrievalDealCmd,
		retrievalDealStateCmd,
	},
}

var retrievalDealsListCmd = &cli.Command{
	Name:  "list",
	Usage: "List all active retrieval deals for this miner",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "receiver",
			Usage: "client peer id",
		},
		&cli.StringFlag{
			Name:  "data-cid",
			Usage: "deal root cid",
		},
		&cli.Uint64Flag{
			Name: "status",
			Usage: `
deal status, show all deal status: ./droplet retrieval deal statuses.
part statuses:
6  DealStatusAccepted
15 DealStatusCompleted
16 DealStatusDealNotFound
17 DealStatusErrored
`,
		},
		offsetFlag,
		limitFlag,
		&cli.BoolFlag{
			Name:  "discard-failed",
			Usage: "filter errored deal",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		params := market.RetrievalDealQueryParams{
			Receiver:   cctx.String("receiver"),
			PayloadCID: cctx.String("data-cid"),
			Page: market.Page{
				Offset: cctx.Int(offsetFlag.Name),
				Limit:  cctx.Int(limitFlag.Name),
			},
			DiscardFailedDeal: cctx.Bool("discard-failed"),
		}
		if cctx.IsSet("status") {
			status := cctx.Uint64("status")
			params.Status = &status
		}

		deals, err := api.MarketListRetrievalDeals(ReqContext(cctx), &params)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

		_, _ = fmt.Fprintf(w, "Receiver\tDealID\tPayload\tState\tPricePerByte\tBytesSent\tPaied\tInterval\tMessage\n")

		for _, deal := range deals {
			payloadCid := deal.PayloadCID.String()
			if !cctx.Bool("verbose") {
				payloadCid = "..." + payloadCid[len(payloadCid)-8:]
			}

			_, _ = fmt.Fprintf(w,
				"%s\t%d\t%s\t%s\t%s\t%d\t%d\t%d\t%s\n",
				deal.Receiver.String(),
				deal.ID,
				payloadCid,
				retrievalmarket.DealStatuses[deal.Status],
				deal.PricePerByte.String(),
				deal.TotalSent,
				deal.FundsReceived,
				deal.CurrentInterval,
				deal.Message,
			)
		}

		return w.Flush()
	},
}

var getRetrievalDealCmd = &cli.Command{
	Name:      "get",
	Usage:     "Print a retrieval deal",
	ArgsUsage: "<receiver> <dealID>",
	Action: func(cliCtx *cli.Context) error {
		api, closer, err := NewMarketNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()

		if cliCtx.NArg() != 2 {
			return fmt.Errorf("expected 2 arguments")
		}

		receiver, err := peer.Decode(cliCtx.Args().First())
		if err != nil {
			return err
		}
		dealID, err := strconv.ParseUint(cliCtx.Args().Get(1), 10, 64)
		if err != nil {
			return err
		}

		ctx := ReqContext(cliCtx)
		deal, err := api.MarketGetRetrievalDeal(ctx, receiver, dealID)
		if err != nil {
			return err
		}

		return outputRetrievalDeal(deal)
	},
}

var retrievalDealStateCmd = &cli.Command{
	Name:  "statuses",
	Usage: "Print all retrieval deal status",
	Action: func(cliCtx *cli.Context) error {
		return printStates(retrievalmarket.DealStatuses)
	},
}

func outputRetrievalDeal(deal *market.ProviderDealState) error {
	var channelID, pieceCID string
	var raw []byte
	var err error
	if deal.ChannelID != nil {
		channelID = deal.ChannelID.String()
	}
	if deal.PieceCID != nil {
		pieceCID = deal.PieceCID.String()
	}
	if !deal.Selector.IsNull() {
		raw, err = json.Marshal(deal.Selector)
		if err != nil {
			return err
		}
	}
	data := []kv{
		{"Receiver", deal.Receiver},
		{"DealID", deal.ID},
		{"PayloadCID", deal.PayloadCID},
		{"Status", retrievalmarket.DealStatuses[deal.Status]},
		{"PricePerByte", deal.PricePerByte.String()},
		{"BytesSent", deal.TotalSent},
		{"Paid", deal.FundsReceived},
		{"Interval", deal.CurrentInterval},
		{"Message", deal.Message},
		{"ChannelID", channelID},
		{"StoreID", deal.StoreID},
		{"SelStorageProposalCid", deal.SelStorageProposalCid},
		{"PieceCID", pieceCID},
		{"PaymentIntervalIncrease", deal.PaymentIntervalIncrease},
		{"UnsealPrice", deal.UnsealPrice},
		{"Selector", raw},
		{"CreatedAt", time.Unix(int64(deal.CreatedAt), 0).Format(time.RFC3339)},
		{"UpdatedAt", time.Unix(int64(deal.UpdatedAt), 0).Format(time.RFC3339)},
	}

	fillSpaceAndPrint(data, len("PaymentIntervalIncrease"))

	return nil
}

var queryProtocols = &cli.Command{
	Name:      "protocols",
	Usage:     "query retrieval support protocols",
	ArgsUsage: "<miner>",
	Action: func(cctx *cli.Context) error {
		if cctx.Args().Len() == 0 {
			return fmt.Errorf("must pass miner")
		}

		api, closer, err := NewFullNode(cctx, OldMarketRepoPath)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		miner, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}
		minerInfo, err := api.StateMinerInfo(ctx, miner, types.EmptyTSK)
		if err != nil {
			return err
		}
		if minerInfo.PeerId == nil {
			return fmt.Errorf("peer id is nil")
		}

		h, err := libp2p.New(
			libp2p.Identity(nil),
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		)
		if err != nil {
			return err
		}

		addrs, err := utils.ConvertMultiaddr(minerInfo.Multiaddrs)
		if err != nil {
			return err
		}
		if err := h.Connect(ctx, peer.AddrInfo{ID: *minerInfo.PeerId, Addrs: addrs}); err != nil {
			return err
		}
		stream, err := h.NewStream(ctx, *minerInfo.PeerId, mtypes.TransportsProtocolID)
		if err != nil {
			return fmt.Errorf("failed to open stream to peer: %w", err)
		}
		_ = stream.SetReadDeadline(time.Now().Add(time.Minute))
		//nolint: errcheck
		defer stream.SetReadDeadline(time.Time{})

		// Read the response from the stream
		queryResponsei, err := mtypes.BindnodeRegistry.TypeFromReader(stream, (*mtypes.QueryResponse)(nil), dagcbor.Decode)
		if err != nil {
			return fmt.Errorf("reading query response: %w", err)
		}
		queryResponse := queryResponsei.(*mtypes.QueryResponse)

		for _, p := range queryResponse.Protocols {
			fmt.Println(p)
		}

		return nil
	},
}
