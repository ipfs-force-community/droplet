package main

import (
	"bytes"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"

	cli2 "github.com/filecoin-project/venus-market/v2/cli"
)

var actorFundsCmd = &cli.Command{
	Name:  "actor-funds",
	Usage: "manage market actor funds",
	Subcommands: []*cli.Command{
		actorFundsBalancesCmd,
		actorFundsAddCmd,
		actorFundsWithdrawCmd,
	},
}

var actorFundsBalancesCmd = &cli.Command{
	Name:  "balances",
	Usage: "Print storage market client balances",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "client",
			Usage: "specify storage client address",
		},
	},
	Action: func(cctx *cli.Context) error {
		fapi, fcloser, err := cli2.NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		var addr address.Address
		if clientFlag := cctx.String("client"); clientFlag != "" {
			ca, err := address.NewFromString(clientFlag)
			if err != nil {
				return err
			}
			addr = ca
		} else {
			def, err := api.DefaultAddress(ctx)
			if err != nil {
				return err
			}
			addr = def
		}

		balance, err := fapi.StateMarketBalance(ctx, addr, types.EmptyTSK)
		if err != nil {
			return err
		}

		reserved, err := api.MarketGetReserved(ctx, addr)
		if err != nil {
			return err
		}

		avail := big.Sub(big.Sub(balance.Escrow, balance.Locked), reserved)
		if avail.LessThan(big.Zero()) {
			avail = big.Zero()
		}

		fmt.Printf("Client Market Balance for address %s:\n", addr)

		fmt.Printf("  Escrowed Funds:        %s\n", types.FIL(balance.Escrow))
		fmt.Printf("  Locked Funds:          %s\n", types.FIL(balance.Locked))
		fmt.Printf("  Reserved Funds:        %s\n", types.FIL(reserved))
		fmt.Printf("  Available to Withdraw: %s\n", types.FIL(avail))

		return nil
	},
}

var actorFundsAddCmd = &cli.Command{
	Name:      "add",
	Usage:     "Add funds to the Storage Market Actor",
	ArgsUsage: "<amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "from",
			Usage:   "Specify address to move funds from, otherwise it will use the default wallet address",
			Aliases: []string{"f"},
		},
		&cli.StringFlag{
			Name:    "address",
			Usage:   "Market address to move funds to (account or miner actor address, defaults to --from address)",
			Aliases: []string{"a"},
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return fmt.Errorf("getting node API: %w", err)
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		// Get amount param
		if !cctx.Args().Present() {
			return fmt.Errorf("must pass amount to add")
		}
		f, err := types.ParseFIL(cctx.Args().First())
		if err != nil {
			return fmt.Errorf("parsing 'amount' argument: %w", err)
		}

		amt := abi.TokenAmount(f)

		// Get from param
		var from address.Address
		if cctx.String("from") != "" {
			from, err = address.NewFromString(cctx.String("from"))
			if err != nil {
				return fmt.Errorf("parsing from address: %w", err)
			}
		} else {
			from, err = api.DefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("getting default wallet address: %w", err)
			}
		}

		// Get address param
		addr := from
		if cctx.String("address") != "" {
			addr, err = address.NewFromString(cctx.String("address"))
			if err != nil {
				return fmt.Errorf("parsing market address: %w", err)
			}
		}

		// Add balance to market actor
		fmt.Printf("Submitting Add Balance message for amount %s for address %s\n", types.FIL(amt), addr)
		smsg, err := api.MarketAddBalance(ctx, from, addr, amt)
		if err != nil {
			return fmt.Errorf("add balance error: %w", err)
		}

		fmt.Printf("AddBalance message cid: %s\n", smsg)

		return nil
	},
}

var actorFundsWithdrawCmd = &cli.Command{
	Name:      "withdraw",
	Usage:     "Withdraw funds from the Storage Market Actor",
	ArgsUsage: "[amount (FIL) optional, otherwise will withdraw max available]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "wallet",
			Usage:   "Specify address to withdraw funds to, otherwise it will use the default wallet address",
			Aliases: []string{"w"},
		},
		&cli.StringFlag{
			Name:    "address",
			Usage:   "Market address to withdraw from (account or miner actor address, defaults to --wallet address)",
			Aliases: []string{"a"},
		},
		&cli.IntFlag{
			Name:  "confidence",
			Usage: "number of block confirmations to wait for",
			Value: int(constants.MessageConfidence),
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return fmt.Errorf("getting node API: %w", err)
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)

		fapi, fcloser, err := cli2.NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer fcloser()

		var wallet address.Address
		if cctx.String("wallet") != "" {
			wallet, err = address.NewFromString(cctx.String("wallet"))
			if err != nil {
				return fmt.Errorf("parsing from address: %w", err)
			}
		} else {
			wallet, err = api.DefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("getting default wallet address: %w", err)
			}
		}

		addr := wallet
		if cctx.String("address") != "" {
			addr, err = address.NewFromString(cctx.String("address"))
			if err != nil {
				return fmt.Errorf("parsing market address: %w", err)
			}
		}

		// Work out if there are enough unreserved, unlocked funds to withdraw
		bal, err := fapi.StateMarketBalance(ctx, addr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting market balance for address %s: %w", addr.String(), err)
		}

		reserved, err := api.MarketGetReserved(ctx, addr)
		if err != nil {
			return fmt.Errorf("getting market reserved amount for address %s: %w", addr.String(), err)
		}

		avail := big.Subtract(big.Subtract(bal.Escrow, bal.Locked), reserved)

		notEnoughErr := func(msg string) error {
			return fmt.Errorf("%s; "+
				"available (%s) = escrow (%s) - locked (%s) - reserved (%s)",
				msg, types.FIL(avail), types.FIL(bal.Escrow), types.FIL(bal.Locked), types.FIL(reserved))
		}

		if avail.IsZero() || avail.LessThan(big.Zero()) {
			avail = big.Zero()
			return notEnoughErr("no funds available to withdraw")
		}

		// Default to withdrawing all available funds
		amt := avail

		// If there was an amount argument, only withdraw that amount
		if cctx.Args().Present() {
			f, err := types.ParseFIL(cctx.Args().First())
			if err != nil {
				return fmt.Errorf("parsing 'amount' argument: %w", err)
			}

			amt = abi.TokenAmount(f)
		}

		// Check the amount is positive
		if amt.IsZero() || amt.LessThan(big.Zero()) {
			return fmt.Errorf("amount must be > 0")
		}

		// Check there are enough available funds
		if amt.GreaterThan(avail) {
			msg := fmt.Sprintf("can't withdraw more funds than available; requested: %s", types.FIL(amt))
			return notEnoughErr(msg)
		}

		fmt.Printf("Submitting WithdrawBalance message for amount %s for address %s\n", types.FIL(amt), wallet.String())
		smsg, err := api.MarketWithdraw(ctx, wallet, addr, amt)
		if err != nil {
			return fmt.Errorf("fund manager withdraw error: %w", err)
		}

		fmt.Printf("WithdrawBalance message cid: %s\n", smsg)

		// wait for it to get mined into a block
		wait, err := fapi.StateWaitMsg(ctx, smsg, uint64(cctx.Int("confidence")), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			fmt.Println(cctx.App.Writer, "withdrawal failed!")
			return err
		}

		nv, err := fapi.StateNetworkVersion(ctx, wait.TipSet)
		if err != nil {
			return err
		}

		if nv >= network.Version14 {
			var withdrawn abi.TokenAmount
			if err := withdrawn.UnmarshalCBOR(bytes.NewReader(wait.Receipt.Return)); err != nil {
				return err
			}

			fmt.Printf("Successfully withdrew %s \n", types.FIL(withdrawn))
			if withdrawn.LessThan(amt) {
				fmt.Printf("Note that this is less than the requested amount of %s \n", types.FIL(amt))
			}
		}

		return nil
	},
}
