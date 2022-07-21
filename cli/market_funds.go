package cli

import (
	"bytes"
	"fmt"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/urfave/cli/v2"
)

var MarketCmds = &cli.Command{
	Name:  "actor-funds",
	Usage: "Interact with market balances",
	Subcommands: []*cli.Command{
		marketBalancesCmd,
		walletMarketAdd,
		walletMarketWithdraw,
	},
}
var marketBalancesCmd = &cli.Command{
	Name:  "balances",
	Usage: "Print storage market client balances",
	Flags: []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		fapi, fcloser, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer fcloser()

		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		addr, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
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

var walletMarketAdd = &cli.Command{
	Name:      "add",
	Usage:     "Add funds to the Storage Market Actor",
	ArgsUsage: "<amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "from",
			Usage:    "Specify address to move funds from, otherwise it will use the default wallet address",
			Aliases:  []string{"f"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "address",
			Usage:    "Market address to move funds to (account or miner actor address, defaults to --from address)",
			Aliases:  []string{"a"},
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		afmt := NewAppFmt(cctx.App)

		// Get amount param
		if !cctx.Args().Present() {
			return fmt.Errorf("must pass amount to add")
		}
		f, err := types.ParseFIL(cctx.Args().First())
		if err != nil {
			return fmt.Errorf("parsing 'amount' argument: %w", err)
		}

		amt := abi.TokenAmount(f)

		// Get address param
		from := cctx.String("from")
		if from == "" {
			return fmt.Errorf("from is empty")
		}
		fromAddr, err := address.NewFromString(from)
		if err != nil {
			return err
		}

		addrStr := cctx.String("address")
		if addrStr == "" {
			return fmt.Errorf("from is empty")
		}
		addr, err := address.NewFromString(addrStr)
		if err != nil {
			return err
		}

		// Add balance to market actor
		fmt.Printf("Submitting Add Balance message for amount %s for address %s\n", types.FIL(amt), fromAddr)
		params, err := actors.SerializeParams(&addr)
		if err != nil {
			return err
		}
		uid, err := api.MessagerPushMessage(cctx.Context, &types.Message{
			Version: 0,
			To:      market.Address,
			From:    fromAddr,
			Nonce:   0,
			Value:   amt,
			Method:  market.Methods.AddBalance,
			Params:  params,
		}, nil)

		if err != nil {
			return err
		}
		fmt.Printf("msg uid is : %s, waiting for the processing result ...\n", uid)

		mw, err := api.MessagerWaitMessage(cctx.Context, uid)
		if err != nil {
			return fmt.Errorf("waiting for worker init: %w", err)
		}

		if mw.Receipt.ExitCode != 0 {
			return fmt.Errorf("msg run failed, exit code %d", mw.Receipt.ExitCode)
		}
		afmt.Printf("AddBalance message cid: %s\n", mw.Message)

		return nil
	},
}

var walletMarketWithdraw = &cli.Command{
	Name:      "withdraw",
	Usage:     "Withdraw funds from the Storage Market Actor",
	ArgsUsage: "[amount (FIL) optional, otherwise will withdraw max available]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "wallet",
			Usage:    "Specify address to withdraw funds to",
			Aliases:  []string{"w"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "address",
			Usage:    "Market address to withdraw from (account or miner actor address, defaults to --wallet address)",
			Aliases:  []string{"a"},
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		api, acloser, err := NewFullNode(cctx)
		if err != nil {
			return err
		}
		defer acloser()

		marketApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		afmt := NewAppFmt(cctx.App)

		wallet, err := address.NewFromString(cctx.String("wallet"))
		if err != nil {
			return fmt.Errorf("parsing from address: %w", err)
		}

		addr, err := address.NewFromString(cctx.String("address"))
		if err != nil {
			return fmt.Errorf("parsing market address: %w", err)
		}

		// Work out if there are enough unreserved, unlocked funds to withdraw
		bal, err := api.StateMarketBalance(ctx, addr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting market balance for address %s: %w", addr.String(), err)
		}

		reserved, err := marketApi.MarketGetReserved(ctx, addr)
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
		msgCid, err := marketApi.MarketWithdraw(ctx, wallet, addr, amt)
		if err != nil {
			return fmt.Errorf("fund manager withdraw error: %w", err)
		}

		afmt.Printf("WithdrawBalance message cid: %s\n", msgCid)

		// wait for it to get mined into a block
		wait, err := marketApi.MessagerWaitMessage(ctx, msgCid)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			afmt.Println(cctx.App.Writer, "withdrawal failed!")
			return err
		}

		nv, err := api.StateNetworkVersion(ctx, wait.TipSet)
		if err != nil {
			return err
		}

		if nv >= network.Version14 {
			var withdrawn abi.TokenAmount
			if err := withdrawn.UnmarshalCBOR(bytes.NewReader(wait.Receipt.Return)); err != nil {
				return err
			}

			afmt.Printf("Successfully withdrew %s \n", types.FIL(withdrawn))
			if withdrawn.LessThan(amt) {
				fmt.Printf("Note that this is less than the requested amount of %s \n", types.FIL(amt))
			}
		}

		return nil
	},
}
