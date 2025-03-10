package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/builtin/v8/paych"
	"github.com/urfave/cli/v2"

	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/paychmgr"

	"github.com/filecoin-project/venus/pkg/constants"
	lpaych "github.com/filecoin-project/venus/venus-shared/actors/builtin/paych"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var paychCmd = &cli.Command{
	Name:  "paych",
	Usage: "Manage payment channels",
	Subcommands: []*cli.Command{
		paychAddFundsCmd,
		paychListCmd,
		paychVoucherCmd,
		paychSettleCmd,
		paychStatusCmd,
		paychStatusByFromToCmd,
		paychCloseCmd,
	},
}

var paychAddFundsCmd = &cli.Command{
	Name:      "add-funds",
	Usage:     "Add funds to the payment channel between fromAddress and toAddress. Creates the payment channel if it doesn't already exist.",
	ArgsUsage: "[fromAddress toAddress amount]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "restart-retrievals",
			Usage: "restart stalled retrieval deals on this payment channel",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "reserve",
			Usage: "mark funds as reserved",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 3 {
			return cli2.IncorrectNumArgs(cctx)
		}

		from, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("failed to parse from address: %s", err))
		}

		to, err := address.NewFromString(cctx.Args().Get(1))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("failed to parse to address: %s", err))
		}

		amt, err := types.ParseFIL(cctx.Args().Get(2))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("parsing amount failed: %s", err))
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
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

		// Send a message to chain to create channel / add funds to existing
		// channel
		var info *types.ChannelInfo
		if cctx.Bool("reserve") {
			info, err = fapi.PaychGet(ctx, from, to, types.BigInt(amt), types.PaychGetOpts{
				OffChain: false,
			})
		} else {
			info, err = fapi.PaychFund(ctx, from, to, types.BigInt(amt))
		}
		if err != nil {
			return err
		}

		// Wait for the message to be confirmed
		fmt.Println("waiting for confirmation..")
		chAddr, err := fapi.PaychGetWaitReady(ctx, info.WaitSentinel)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cctx.App.Writer, chAddr) // nolint:errcheck
		restartRetrievals := cctx.Bool("restart-retrievals")
		if restartRetrievals {
			return api.ClientRetrieveTryRestartInsufficientFunds(ctx, chAddr)
		}
		return nil
	},
}

var paychStatusByFromToCmd = &cli.Command{
	Name:      "status-by-from-to",
	Usage:     "Show the status of an active outbound payment channel by from/to addresses",
	ArgsUsage: "[fromAddress toAddress]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.IncorrectNumArgs(cctx)
		}
		ctx := cli2.ReqContext(cctx)

		from, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("failed to parse from address: %s", err))
		}

		to, err := address.NewFromString(cctx.Args().Get(1))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("failed to parse to address: %s", err))
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		avail, err := fapi.PaychAvailableFundsByFromTo(ctx, from, to)
		if err != nil {
			return err
		}

		paychStatus(cctx.App.Writer, avail)
		return nil
	},
}

var paychStatusCmd = &cli.Command{
	Name:      "status",
	Usage:     "Show the status of an outbound payment channel",
	ArgsUsage: "[channelAddress]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return cli2.IncorrectNumArgs(cctx)
		}
		ctx := cli2.ReqContext(cctx)

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("failed to parse channel address: %s", err))
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		avail, err := fapi.PaychAvailableFunds(ctx, ch)
		if err != nil {
			return err
		}

		paychStatus(cctx.App.Writer, avail)
		return nil
	},
}

func paychStatus(writer io.Writer, avail *types.ChannelAvailableFunds) {
	if avail.Channel == nil {
		if avail.PendingWaitSentinel != nil {
			_, _ = fmt.Fprint(writer, "Creating channel\n")                                  // nolint:errcheck
			_, _ = fmt.Fprintf(writer, "  From:          %s\n", avail.From)                  // nolint:errcheck
			_, _ = fmt.Fprintf(writer, "  To:            %s\n", avail.To)                    // nolint:errcheck
			_, _ = fmt.Fprintf(writer, "  Pending Amt:   %s\n", types.FIL(avail.PendingAmt)) // nolint:errcheck
			_, _ = fmt.Fprintf(writer, "  Wait Sentinel: %s\n", avail.PendingWaitSentinel)   // nolint:errcheck
			return
		}
		_, _ = fmt.Fprint(writer, "Channel does not exist\n")  // nolint:errcheck
		_, _ = fmt.Fprintf(writer, "  From: %s\n", avail.From) // nolint:errcheck
		_, _ = fmt.Fprintf(writer, "  To:   %s\n", avail.To)   // nolint:errcheck
		return
	}

	if avail.PendingWaitSentinel != nil {
		_, _ = fmt.Fprint(writer, "Adding Funds to channel\n") // nolint:errcheck
	} else {
		_, _ = fmt.Fprint(writer, "Channel exists\n") // nolint:errcheck
	}

	nameValues := [][]string{
		{"Channel", avail.Channel.String()},
		{"From", avail.From.String()},
		{"To", avail.To.String()},
		{"Confirmed Amt", fmt.Sprintf("%s", types.FIL(avail.ConfirmedAmt))},
		{"Available Amt", fmt.Sprintf("%s", types.FIL(avail.NonReservedAmt))},
		{"Voucher Redeemed Amt", fmt.Sprintf("%s", types.FIL(avail.VoucherReedeemedAmt))},
		{"Pending Amt", fmt.Sprintf("%s", types.FIL(avail.PendingAmt))},
		{"Pending Available Amt", fmt.Sprintf("%s", types.FIL(avail.PendingAvailableAmt))},
		{"Queued Amt", fmt.Sprintf("%s", types.FIL(avail.QueuedAmt))},
	}
	if avail.PendingWaitSentinel != nil {
		nameValues = append(nameValues, []string{
			"Add Funds Wait Sentinel",
			avail.PendingWaitSentinel.String(),
		})
	}
	_, _ = fmt.Fprint(writer, formatNameValues(nameValues)) // nolint:errcheck
}

func formatNameValues(nameValues [][]string) string {
	maxLen := 0
	for _, nv := range nameValues {
		if len(nv[0]) > maxLen {
			maxLen = len(nv[0])
		}
	}
	out := make([]string, len(nameValues))
	for i, nv := range nameValues {
		namePad := strings.Repeat(" ", maxLen-len(nv[0]))
		out[i] = "  " + nv[0] + ": " + namePad + nv[1]
	}
	return strings.Join(out, "\n") + "\n"
}

var paychListCmd = &cli.Command{
	Name:  "list",
	Usage: "List all locally registered payment channels",
	Action: func(cctx *cli.Context) error {
		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		chs, err := fapi.PaychList(ctx)
		if err != nil {
			return err
		}

		for _, v := range chs {
			_, _ = fmt.Fprintln(cctx.App.Writer, v.String()) // nolint:errcheck
		}
		return nil
	},
}

var paychSettleCmd = &cli.Command{
	Name:      "settle",
	Usage:     "Settle a payment channel",
	ArgsUsage: "[channelAddress]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("failed to parse payment channel address: %s", err)
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		mcid, err := fapi.PaychSettle(ctx, ch)
		if err != nil {
			return err
		}

		mwait, err := fapi.StateWaitMsg(ctx, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if mwait.Receipt.ExitCode != 0 {
			return fmt.Errorf("settle message execution failed (exit code %d)", mwait.Receipt.ExitCode)
		}

		_, _ = fmt.Fprintf(cctx.App.Writer, "Settled channel %s\n", ch) // nolint:errcheck
		return nil
	},
}

var paychCloseCmd = &cli.Command{
	Name:      "collect",
	Usage:     "Collect funds for a payment channel",
	ArgsUsage: "[channelAddress]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return fmt.Errorf("failed to parse payment channel address: %s", err)
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		mcid, err := fapi.PaychCollect(ctx, ch)
		if err != nil {
			return err
		}

		mwait, err := fapi.StateWaitMsg(ctx, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return nil
		}
		if mwait.Receipt.ExitCode != 0 {
			return fmt.Errorf("collect message execution failed (exit code %d)", mwait.Receipt.ExitCode)
		}

		_, _ = fmt.Fprintf(cctx.App.Writer, "Collected funds for channel %s\n", ch) // nolint:errcheck
		return nil
	},
}

var paychVoucherCmd = &cli.Command{
	Name:  "voucher",
	Usage: "Interact with payment channel vouchers",
	Subcommands: []*cli.Command{
		paychVoucherCreateCmd,
		paychVoucherCheckCmd,
		paychVoucherAddCmd,
		paychVoucherListCmd,
		paychVoucherBestSpendableCmd,
		paychVoucherSubmitCmd,
	},
}

var paychVoucherCreateCmd = &cli.Command{
	Name:      "create",
	Usage:     "Create a signed payment channel voucher",
	ArgsUsage: "[channelAddress amount]",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "lane",
			Value: 0,
			Usage: "specify payment channel lane to use",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		amt, err := types.ParseFIL(cctx.Args().Get(1))
		if err != nil {
			return cli2.ShowHelp(cctx, fmt.Errorf("parsing amount failed: %s", err))
		}

		lane := cctx.Int("lane")

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		v, err := fapi.PaychVoucherCreate(ctx, ch, types.BigInt(amt), uint64(lane))
		if err != nil {
			return err
		}

		if v.Voucher == nil {
			return fmt.Errorf("could not create voucher: insufficient funds in channel, shortfall: %d", v.Shortfall)
		}

		enc, err := EncodedString(v.Voucher)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cctx.App.Writer, enc) // nolint:errcheck
		return nil
	},
}

var paychVoucherCheckCmd = &cli.Command{
	Name:      "check",
	Usage:     "Check validity of payment channel voucher",
	ArgsUsage: "[channelAddress voucher]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		sv, err := lpaych.DecodeSignedVoucher(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		if err := fapi.PaychVoucherCheckValid(ctx, ch, sv); err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cctx.App.Writer, "voucher is valid") // nolint:errcheck
		return nil
	},
}

var paychVoucherAddCmd = &cli.Command{
	Name:      "add",
	Usage:     "Add payment channel voucher to local datastore",
	ArgsUsage: "[channelAddress voucher]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		sv, err := lpaych.DecodeSignedVoucher(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		// TODO: allow passing proof bytes
		if _, err := fapi.PaychVoucherAdd(ctx, ch, sv, nil, types.NewInt(0)); err != nil {
			return err
		}

		return nil
	},
}

var paychVoucherListCmd = &cli.Command{
	Name:      "list",
	Usage:     "List stored vouchers for a given payment channel",
	ArgsUsage: "[channelAddress]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "export",
			Usage: "Print voucher as serialized string",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		vouchers, err := fapi.PaychVoucherList(ctx, ch)
		if err != nil {
			return err
		}

		for _, v := range sortVouchers(vouchers) {
			export := cctx.Bool("export")
			err := outputVoucher(cctx.App.Writer, v, export)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var paychVoucherBestSpendableCmd = &cli.Command{
	Name:      "best-spendable",
	Usage:     "Print vouchers with highest value that is currently spendable for each lane",
	ArgsUsage: "[channelAddress]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "export",
			Usage: "Print voucher as serialized string",
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		vouchersByLane, err := paychmgr.BestSpendableByLane(ctx, fapi, ch)
		if err != nil {
			return err
		}

		var vouchers []*paych.SignedVoucher
		for _, vchr := range vouchersByLane {
			vouchers = append(vouchers, vchr)
		}
		for _, best := range sortVouchers(vouchers) {
			export := cctx.Bool("export")
			err := outputVoucher(cctx.App.Writer, best, export)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func sortVouchers(vouchers []*paych.SignedVoucher) []*paych.SignedVoucher {
	sort.Slice(vouchers, func(i, j int) bool {
		if vouchers[i].Lane == vouchers[j].Lane {
			return vouchers[i].Nonce < vouchers[j].Nonce
		}
		return vouchers[i].Lane < vouchers[j].Lane
	})
	return vouchers
}

func outputVoucher(w io.Writer, v *paych.SignedVoucher, export bool) error {
	var enc string
	if export {
		var err error
		enc, err = EncodedString(v)
		if err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintf(w, "Lane %d, Nonce %d: %s", v.Lane, v.Nonce, types.FIL(v.Amount)) // nolint:errcheck
	if export {
		_, _ = fmt.Fprintf(w, "; %s", enc) // nolint:errcheck
	}
	_, _ = fmt.Fprintln(w) // nolint:errcheck
	return nil
}

var paychVoucherSubmitCmd = &cli.Command{
	Name:      "submit",
	Usage:     "Submit voucher to chain to update payment channel state",
	ArgsUsage: "[channelAddress voucher]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.IncorrectNumArgs(cctx)
		}

		ch, err := address.NewFromString(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		sv, err := lpaych.DecodeSignedVoucher(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cctx)

		mcid, err := fapi.PaychVoucherSubmit(ctx, ch, sv, nil, nil)
		if err != nil {
			return err
		}

		mwait, err := fapi.StateWaitMsg(ctx, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if mwait.Receipt.ExitCode != 0 {
			return fmt.Errorf("message execution failed (exit code %d)", mwait.Receipt.ExitCode)
		}

		_, _ = fmt.Fprintln(cctx.App.Writer, "channel updated successfully") // nolint:errcheck

		return nil
	},
}

func EncodedString(sv *paych.SignedVoucher) (string, error) {
	buf := new(bytes.Buffer)
	if err := sv.MarshalCBOR(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}
