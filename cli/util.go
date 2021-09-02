package cli

import (
	"context"
	"fmt"
	"github.com/docker/go-units"
	"github.com/fatih/color"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/api"
	impl2 "github.com/filecoin-project/venus-market/api/impl"
	"github.com/filecoin-project/venus-market/cli/tablewriter"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/urfave/cli/v2"
	"io"
	"os"
	"os/signal"
	"path"
	"sort"
	"syscall"
)

func NewMarketNode(cctx *cli.Context) (api.MarketFullNode, jsonrpc.ClientCloser, error) {
	cfgPath := path.Join(cctx.String("repo"), "config.toml")
	marketCfg := &config.MarketConfig{}
	err := config.LoadConfig(cfgPath, marketCfg)
	if err != nil {
		return nil, nil, err
	}
	apiInfo := apiinfo.NewAPIInfo(marketCfg.API.ListenAddress, marketCfg.API.Token)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	impl := &impl2.MarketNodeImpl{}
	closer, err := jsonrpc.NewMergeClient(cctx.Context, addr, "VENUS_MARKET", []interface{}{impl}, apiInfo.AuthHeader())
	if err != nil {
		return nil, nil, err
	}
	return impl, closer, nil
}

func NewFullNode(cctx *cli.Context) (apiface.FullNode, jsonrpc.ClientCloser, error) {
	cfgPath := path.Join(cctx.String("repo"), "config.toml")
	marketCfg := &config.MarketConfig{}
	err := config.LoadConfig(cfgPath, marketCfg)
	if err != nil {
		return nil, nil, err
	}
	apiInfo := apiinfo.NewAPIInfo(marketCfg.Node.Url, marketCfg.Node.Token)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	impl := &client.FullNodeStruct{}
	closer, err := jsonrpc.NewMergeClient(cctx.Context, addr, "VENUS_MARKET", utils.GetInternalStructs(impl), apiInfo.AuthHeader())
	if err != nil {
		return nil, nil, err
	}
	return impl, closer, nil
}

// OutputDataTransferChannels generates table output for a list of channels
func OutputDataTransferChannels(out io.Writer, channels []types.DataTransferChannel, verbose, completed, color, showFailed bool) {
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].TransferID < channels[j].TransferID
	})

	var receivingChannels, sendingChannels []types.DataTransferChannel
	for _, channel := range channels {
		if !completed && channel.Status == datatransfer.Completed {
			continue
		}
		if !showFailed && (channel.Status == datatransfer.Failed || channel.Status == datatransfer.Cancelled) {
			continue
		}
		if channel.IsSender {
			sendingChannels = append(sendingChannels, channel)
		} else {
			receivingChannels = append(receivingChannels, channel)
		}
	}

	fmt.Fprintf(out, "Sending Channels\n\n")
	w := tablewriter.New(tablewriter.Col("ID"),
		tablewriter.Col("Status"),
		tablewriter.Col("Sending To"),
		tablewriter.Col("Root Cid"),
		tablewriter.Col("Initiated?"),
		tablewriter.Col("Transferred"),
		tablewriter.Col("Voucher"),
		tablewriter.NewLineCol("Message"))
	for _, channel := range sendingChannels {
		w.Write(toChannelOutput(color, "Sending To", channel, verbose))
	}
	w.Flush(out) //nolint:errcheck

	fmt.Fprintf(out, "\nReceiving Channels\n\n")
	w = tablewriter.New(tablewriter.Col("ID"),
		tablewriter.Col("Status"),
		tablewriter.Col("Receiving From"),
		tablewriter.Col("Root Cid"),
		tablewriter.Col("Initiated?"),
		tablewriter.Col("Transferred"),
		tablewriter.Col("Voucher"),
		tablewriter.NewLineCol("Message"))
	for _, channel := range receivingChannels {
		w.Write(toChannelOutput(color, "Receiving From", channel, verbose))
	}
	w.Flush(out) //nolint:errcheck
}

func toChannelOutput(useColor bool, otherPartyColumn string, channel types.DataTransferChannel, verbose bool) map[string]interface{} {
	rootCid := channel.BaseCID.String()
	otherParty := channel.OtherPeer.String()
	if !verbose {
		rootCid = ellipsis(rootCid, 8)
		otherParty = ellipsis(otherParty, 8)
	}

	initiated := "N"
	if channel.IsInitiator {
		initiated = "Y"
	}

	voucher := channel.Voucher
	if len(voucher) > 40 && !verbose {
		voucher = ellipsis(voucher, 37)
	}

	return map[string]interface{}{
		"ID":             channel.TransferID,
		"Status":         channelStatusString(useColor, channel.Status),
		otherPartyColumn: otherParty,
		"Root Cid":       rootCid,
		"Initiated?":     initiated,
		"Transferred":    units.BytesSize(float64(channel.Transferred)),
		"Voucher":        voucher,
		"Message":        channel.Message,
	}
}

func ellipsis(s string, length int) string {
	if length > 0 && len(s) > length {
		return "..." + s[len(s)-length:]
	}
	return s
}

func channelStatusString(useColor bool, status datatransfer.Status) string {
	s := datatransfer.Statuses[status]
	if !useColor {
		return s
	}

	switch status {
	case datatransfer.Failed, datatransfer.Cancelled:
		return color.RedString(s)
	case datatransfer.Completed:
		return color.GreenString(s)
	default:
		return s
	}
}

func DaemonContext(cctx *cli.Context) context.Context {
	return context.Background()
}

// ReqContext returns context for cli execution. Calling it for the first time
// installs SIGTERM handler that will close returned context.
// Not safe for concurrent execution.
func ReqContext(cctx *cli.Context) context.Context {
	tCtx := DaemonContext(cctx)

	ctx, done := context.WithCancel(tCtx)
	sigChan := make(chan os.Signal, 2)
	go func() {
		<-sigChan
		done()
	}()
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	return ctx
}

type PrintHelpErr struct {
	Err error
	Ctx *cli.Context
}

func (e *PrintHelpErr) Error() string {
	return e.Err.Error()
}

func (e *PrintHelpErr) Unwrap() error {
	return e.Err
}

func (e *PrintHelpErr) Is(o error) bool {
	_, ok := o.(*PrintHelpErr)
	return ok
}

func ShowHelp(cctx *cli.Context, err error) error {
	return &PrintHelpErr{Err: err, Ctx: cctx}
}
