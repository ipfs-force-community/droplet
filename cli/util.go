package cli

import (
	"context"
	"fmt"
	"github.com/docker/go-units"
	"github.com/fatih/color"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/cli/tablewriter"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"sort"
	"strings"
	"syscall"
)

func NewMarketNode(cctx *cli.Context) (api.MarketFullNode, jsonrpc.ClientCloser, error) {
	homePath, err := homedir.Expand(cctx.String("repo"))
	if err != nil {
		return nil, nil, err
	}
	fmt.Println(homePath)
	apiUrl, err := ioutil.ReadFile(path.Join(homePath, "api"))
	if err != nil {
		return nil, nil, err
	}

	token, err := ioutil.ReadFile(path.Join(homePath, "token"))
	if err != nil {
		return nil, nil, err
	}
	apiInfo := apiinfo.NewAPIInfo(string(apiUrl), string(token))
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	impl := &api.MarketFullNodeStruct{}
	closer, err := jsonrpc.NewMergeClient(cctx.Context, addr, "VENUS_MARKET", []interface{}{impl}, apiInfo.AuthHeader())
	if err != nil {
		return nil, nil, err
	}
	return impl, closer, nil
}

func NewMarketClientNode(cctx *cli.Context) (api.MarketClientNode, jsonrpc.ClientCloser, error) {
	homePath, err := homedir.Expand(cctx.String("repo"))
	if err != nil {
		return nil, nil, err
	}
	fmt.Println(homePath)
	apiUrl, err := ioutil.ReadFile(path.Join(homePath, "api"))
	if err != nil {
		return nil, nil, err
	}

	token, err := ioutil.ReadFile(path.Join(homePath, "token"))
	if err != nil {
		return nil, nil, err
	}
	apiInfo := apiinfo.NewAPIInfo(string(apiUrl), string(token))
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	impl := &api.MarketClientNodeStruct{}
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
	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}

	impl := &client.FullNodeStruct{}
	closer, err := jsonrpc.NewMergeClient(cctx.Context, addr, "Filecoin", utils.GetInternalStructs(impl), apiInfo.AuthHeader())
	if err != nil {
		return nil, nil, err
	}
	return impl, closer, nil
}

func WithCategory(cat string, cmd *cli.Command) *cli.Command {
	cmd.Category = strings.ToUpper(cat)
	return cmd
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

type AppFmt struct {
	app   *cli.App
	Stdin io.Reader
}

func NewAppFmt(a *cli.App) *AppFmt {
	var stdin io.Reader
	istdin, ok := a.Metadata["stdin"]
	if ok {
		stdin = istdin.(io.Reader)
	} else {
		stdin = os.Stdin
	}
	return &AppFmt{app: a, Stdin: stdin}
}

func (a *AppFmt) Print(args ...interface{}) {
	fmt.Fprint(a.app.Writer, args...)
}

func (a *AppFmt) Println(args ...interface{}) {
	fmt.Fprintln(a.app.Writer, args...)
}

func (a *AppFmt) Printf(fmtstr string, args ...interface{}) {
	fmt.Fprintf(a.app.Writer, fmtstr, args...)
}

func (a *AppFmt) Scan(args ...interface{}) (int, error) {
	return fmt.Fscan(a.Stdin, args...)
}
