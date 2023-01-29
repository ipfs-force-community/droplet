package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/docker/go-units"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/urfave/cli/v2"
)

var retirevalAsksCmds = &cli.Command{
	Name:  "ask",
	Usage: "Configure retrieval asks",
	Subcommands: []*cli.Command{
		retrievalGetAskCmd,
		retrievalSetAskCmd,
	},
}

var retrievalSetAskCmd = &cli.Command{
	Name:      "set",
	ArgsUsage: "<miner address>",
	Usage:     "Configure(set/update)the provider's retrieval ask",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "price",
			Usage: "Set the price of the ask for retrievals (FIL/GiB)",
			Value: "0",
		},
		&cli.StringFlag{
			Name:  "unseal-price",
			Usage: "Set the price to unseal",
			Value: "0",
		},
		&cli.StringFlag{
			Name:  "payment-interval",
			Usage: "Set the payment interval (in bytes) for retrieval",
			Value: "1MiB",
		},
		&cli.StringFlag{
			Name:  "payment-interval-increase",
			Usage: "Set the payment interval increase (in bytes) for retrieval",
			Value: "1MiB",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		isUpdate := true
		ask, err := api.MarketGetRetrievalAsk(ctx, mAddr)
		if err != nil {
			if !strings.Contains(err.Error(), "record not found") {
				return err
			}
			ask = &retrievalmarket.Ask{}
			isUpdate = false
		}

		if isUpdate {
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
		} else {
			price, err := types.ParseFIL(cctx.String("price"))
			if err != nil {
				return err
			}
			ask.PricePerByte = types.BigDiv(types.BigInt(price), types.NewInt(1<<30))

			unsealPrice, err := types.ParseFIL(cctx.String("unseal-price"))
			if err != nil {
				return err
			}
			ask.UnsealPrice = abi.TokenAmount(unsealPrice)

			paymentInterval, err := units.RAMInBytes(cctx.String("payment-interval"))
			if err != nil {
				return err
			}
			ask.PaymentInterval = uint64(paymentInterval)

			paymentIntervalIncrease, err := units.RAMInBytes(cctx.String("payment-interval-increase"))
			if err != nil {
				return err
			}
			ask.PaymentIntervalIncrease = uint64(paymentIntervalIncrease)
		}

		return api.MarketSetRetrievalAsk(ctx, mAddr, ask)
	},
}

var retrievalGetAskCmd = &cli.Command{
	Name:      "get",
	ArgsUsage: "<miner address>",
	Usage:     "Get the provider's current retrieval ask",
	Action: func(cctx *cli.Context) error {
		ctx := DaemonContext(cctx)

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		if cctx.NArg() != 1 {
			return errors.New("must specify one argument as miner address")
		}
		mAddr, err := address.NewFromString(cctx.Args().Get(0))
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
