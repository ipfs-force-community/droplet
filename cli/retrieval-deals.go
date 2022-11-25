package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/filecoin-project/go-address"

	"github.com/docker/go-units"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/venus/venus-shared/types"
)

var RetrievalDealsCmd = &cli.Command{
	Name:  "retrieval-deals",
	Usage: "Manage retrieval deals and related configuration",
	Subcommands: []*cli.Command{
		retrievalDealSelectionCmd,
		retrievalDealsListCmd,
		retrievalSetAskCmd,
		retrievalGetAskCmd,
	},
}

var retrievalDealSelectionCmd = &cli.Command{
	Name:  "selection",
	Usage: "Configure acceptance criteria for retrieval deal proposals",
	Subcommands: []*cli.Command{
		retrievalDealSelectionShowCmd,
		retrievalDealSelectionResetCmd,
		retrievalDealSelectionRejectCmd,
	},
}

var retrievalDealSelectionShowCmd = &cli.Command{
	Name:  "list",
	Usage: "List retrieval deal proposal selection criteria",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		onlineOk, err := smapi.DealsConsiderOnlineRetrievalDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		offlineOk, err := smapi.DealsConsiderOfflineRetrievalDeals(DaemonContext(cctx), mAddr)
		if err != nil {
			return err
		}

		fmt.Printf("considering online retrieval deals: %t\n", onlineOk)
		fmt.Printf("considering offline retrieval deals: %t\n", offlineOk)

		return nil
	},
}

var retrievalDealSelectionResetCmd = &cli.Command{
	Name:  "reset",
	Usage: "Reset retrieval deal proposal selection criteria to default values",
	Flags: []cli.Flag{
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		err = smapi.DealsSetConsiderOnlineRetrievalDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		err = smapi.DealsSetConsiderOfflineRetrievalDeals(DaemonContext(cctx), mAddr, true)
		if err != nil {
			return err
		}

		return nil
	},
}

var retrievalDealSelectionRejectCmd = &cli.Command{
	Name:  "reject",
	Usage: "Configure criteria which necessitate automatic rejection",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "online",
		},
		&cli.BoolFlag{
			Name: "offline",
		},
		minerFlag,
	},
	Action: func(cctx *cli.Context) error {
		mAddr, err := shouldAddress(cctx.String("miner"), false, false)
		if err != nil {
			return fmt.Errorf("invalid miner address: %w", err)
		}

		smapi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.Bool("online") {
			err = smapi.DealsSetConsiderOnlineRetrievalDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		if cctx.Bool("offline") {
			err = smapi.DealsSetConsiderOfflineRetrievalDeals(DaemonContext(cctx), mAddr, false)
			if err != nil {
				return err
			}
		}

		return nil
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

var retrievalSetAskCmd = &cli.Command{
	Name:  "set-ask",
	Usage: "Configure the provider's retrieval ask",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "price",
			Usage: "Set the price of the ask for retrievals (FIL/GiB)",
		},
		&cli.StringFlag{
			Name:  "unseal-price",
			Usage: "Set the price to unseal",
		},
		&cli.StringFlag{
			Name:        "payment-interval",
			Usage:       "Set the payment interval (in bytes) for retrieval",
			DefaultText: "1MiB",
		},
		&cli.StringFlag{
			Name:        "payment-interval-increase",
			Usage:       "Set the payment interval increase (in bytes) for retrieval",
			DefaultText: "1MiB",
		},
		&cli.StringFlag{
			Name:     "payment-addr",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		mAddr, err := address.NewFromString(cctx.String("payment-addr"))
		if err != nil {
			return err
		}

		ask, err := api.MarketGetRetrievalAsk(ctx, mAddr)
		if err != nil {
			if err.Error() != "record not found" {
				return err
			}
			ask = &retrievalmarket.Ask{}
		}

		if cctx.IsSet("price") {
			v, err := types.ParseFIL(cctx.String("price"))
			if err != nil {
				return err
			}
			ask.PricePerByte = types.BigDiv(types.BigInt(v), types.NewInt(1<<30))
		}

		if cctx.IsSet("unseal-price") {
			v, err := types.ParseFIL(cctx.String("unseal-price"))
			if err != nil {
				return err
			}
			ask.UnsealPrice = abi.TokenAmount(v)
		}

		if cctx.IsSet("payment-interval") {
			v, err := units.RAMInBytes(cctx.String("payment-interval"))
			if err != nil {
				return err
			}
			ask.PaymentInterval = uint64(v)
		}

		if cctx.IsSet("payment-interval-increase") {
			v, err := units.RAMInBytes(cctx.String("payment-interval-increase"))
			if err != nil {
				return err
			}
			ask.PaymentIntervalIncrease = uint64(v)
		}

		return api.MarketSetRetrievalAsk(ctx, mAddr, ask)
	},
}

var retrievalGetAskCmd = &cli.Command{
	Name:  "get-ask",
	Usage: "Get the provider's current retrieval ask",
	Flags: []cli.Flag{
		requiredMinerFlag,
	},
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		mAddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return err
		}

		ask, err := api.MarketGetRetrievalAsk(ctx, mAddr)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintf(w, "Price per Byte\tUnseal Price\tPayment Interval\tPayment Interval Increase\n")
		if ask == nil {
			fmt.Fprintf(w, "<miner does not have an retrieval ask set>\n")
			return w.Flush()
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			types.FIL(ask.PricePerByte),
			types.FIL(ask.UnsealPrice),
			units.BytesSize(float64(ask.PaymentInterval)),
			units.BytesSize(float64(ask.PaymentIntervalIncrease)),
		)
		return w.Flush()
	},
}
