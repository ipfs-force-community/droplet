package cli

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"sort"
	"strings"
	"syscall"

	"github.com/docker/go-units"
	"github.com/fatih/color"
	"github.com/howeyc/gopass"
	"github.com/ipfs/go-cidutil/cidenc"
	"github.com/mitchellh/go-homedir"
	"github.com/multiformats/go-multibase"
	"github.com/urfave/cli/v2"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/v2/cli/tablewriter"
	"github.com/filecoin-project/venus-market/v2/config"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	marketapi "github.com/filecoin-project/venus/venus-shared/api/market"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
)

var CidBaseFlag = cli.StringFlag{
	Name:        "cid-base",
	Hidden:      true,
	Value:       "base32",
	Usage:       "Multibase encoding used for version 1 CIDs in output.",
	DefaultText: "base32",
}

const (
	API_NAMESPACE_VENUS_MARKET  = "VENUS_MARKET"        //nolint
	API_NAMESPACE_MARKET_CLIENT = "VENUS_MARKET_CLIENT" //nolint
)

// GetCidEncoder returns an encoder using the `cid-base` flag if provided, or
// the default (Base32) encoder if not.
func GetCidEncoder(cctx *cli.Context) (cidenc.Encoder, error) {
	val := cctx.String("cid-base")

	e := cidenc.Encoder{Base: multibase.MustNewEncoder(multibase.Base32)}

	if val != "" {
		var err error
		e.Base, err = multibase.EncoderByName(val)
		if err != nil {
			return e, err
		}
	}

	return e, nil
}

var minerFlag = &cli.StringFlag{
	Name: "miner",
}

var requiredMinerFlag = &cli.StringFlag{
	Name:     "miner",
	Required: true,
}

func NewMarketNode(cctx *cli.Context) (marketapi.IMarket, jsonrpc.ClientCloser, error) {
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

	return marketapi.NewIMarketRPC(cctx.Context, addr, apiInfo.AuthHeader())
}

func NewMarketClientNode(cctx *cli.Context) (clientapi.IMarketClient, jsonrpc.ClientCloser, error) {
	homePath, err := homedir.Expand(cctx.String("repo"))
	if err != nil {
		return nil, nil, err
	}
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

	return clientapi.NewIMarketClientRPC(cctx.Context, addr, apiInfo.AuthHeader())
}

func NewFullNode(cctx *cli.Context) (v1api.FullNode, jsonrpc.ClientCloser, error) {
	cfgPath := path.Join(cctx.String("repo"), "config.toml")
	marketCfg := config.DefaultMarketConfig
	err := config.LoadConfig(cfgPath, marketCfg)
	if err != nil {
		return nil, nil, err
	}
	apiInfo := apiinfo.NewAPIInfo(marketCfg.Node.Url, marketCfg.Node.Token)
	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}

	return v1api.NewFullNodeRPC(cctx.Context, addr, apiInfo.AuthHeader())
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
	Stdin gopass.FdReader
}

func NewAppFmt(a *cli.App) *AppFmt {
	var stdin gopass.FdReader
	istdin, ok := a.Metadata["stdin"]
	if ok {
		stdin = istdin.(gopass.FdReader)
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

func (a *AppFmt) GetScret(prompt string, isMasked bool) (string, error) {
	pw, err := gopass.GetPasswdPrompt(prompt, isMasked, a.Stdin, a.app.Writer)
	if err != nil {
		return "", err
	}
	return string(pw), nil
}
