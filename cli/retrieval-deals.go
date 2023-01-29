package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
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
