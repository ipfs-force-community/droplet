package cli

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus/venus-shared/types/market"
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
	},
}

var retrievalDealsCmds = &cli.Command{
	Name:  "deal",
	Usage: "Manage retrieval deals and related configuration",
	Subcommands: []*cli.Command{
		retrievalDealsListCmd,
		getRetrievalDealCmd,
	},
}

var retrievalDealsListCmd = &cli.Command{
	Name:  "list",
	Usage: "List all active retrieval deals for this miner",
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		deals, err := api.MarketListRetrievalDeals(DaemonContext(cctx))
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

		_, _ = fmt.Fprintf(w, "Receiver\tDealID\tPayload\tState\tPricePerByte\tBytesSent\tPaied\tInterval\tMessage\n")

		for _, deal := range deals {
			payloadCid := deal.PayloadCID.String()

			_, _ = fmt.Fprintf(w,
				"%s\t%d\t%s\t%s\t%s\t%d\t%d\t%d\t%s\n",
				deal.Receiver.String(),
				deal.ID,
				"..."+payloadCid[len(payloadCid)-8:],
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

func outputRetrievalDeal(deal *market.ProviderDealState) error {
	var channelID, pieceCID string
	var raw []byte
	if deal.ChannelID != nil {
		channelID = deal.ChannelID.String()
	}
	if deal.PieceCID != nil {
		pieceCID = deal.PieceCID.String()
	}
	if deal.Selector != nil {
		raw = deal.Selector.Raw
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

	maxLen := len("PaymentIntervalIncrease")
	for _, d := range data {
		for i := len(d.k); i < maxLen; i++ {
			d.k += " "
		}
		fmt.Println(d.k, d.v)
	}

	return nil
}
