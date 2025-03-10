package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/go-units"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/howeyc/gopass"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-cidutil/cidenc"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mitchellh/go-homedir"
	"github.com/multiformats/go-multibase"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v9/market"
	"github.com/filecoin-project/go-state-types/crypto"
	crypto2 "github.com/libp2p/go-libp2p/core/crypto"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/ipfs-force-community/droplet/v2/api/clients/signer"
	"github.com/ipfs-force-community/droplet/v2/cli/tablewriter"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/utils"

	"github.com/filecoin-project/venus/venus-shared/api"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	marketapi "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
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

const (
	OldMarketRepoPath = "~/.venusmarket"
	DefMarketRepoPath = "~/.droplet"
)

const (
	OldClientRepoPath = "~/.marketclient"
	DefClientRepoPath = "~/.droplet-client"
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

func NewMarketNode(cctx *cli.Context) (marketapi.IMarket, jsonrpc.ClientCloser, error) {
	homePath, err := GetRepoPath(cctx, "repo", OldMarketRepoPath)
	if err != nil {
		return nil, nil, err
	}

	apiUrl, err := os.ReadFile(path.Join(homePath, "api"))
	if err != nil {
		return nil, nil, err
	}

	token, err := os.ReadFile(path.Join(homePath, "token"))
	if err != nil {
		return nil, nil, err
	}
	apiInfo := api.NewAPIInfo(string(apiUrl), string(token))
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	return marketapi.NewIMarketRPC(cctx.Context, addr, apiInfo.AuthHeader())
}

func GetMarketConfig(cctx *cli.Context) (*config.MarketConfig, error) {
	homePath, err := GetRepoPath(cctx, "repo", OldMarketRepoPath)
	if err != nil {
		return nil, err
	}

	cfgPath := path.Join(homePath, "config.toml")
	marketCfg := config.DefaultMarketConfig
	err = config.LoadConfig(cfgPath, marketCfg)
	if err != nil {
		return nil, err
	}
	return marketCfg, nil
}

func NewMarketClientNode(cctx *cli.Context) (clientapi.IMarketClient, jsonrpc.ClientCloser, error) {
	homePath, err := GetRepoPath(cctx, "repo", OldClientRepoPath)
	if err != nil {
		return nil, nil, err
	}
	apiUrl, err := os.ReadFile(path.Join(homePath, "api"))
	if err != nil {
		return nil, nil, err
	}

	token, err := os.ReadFile(path.Join(homePath, "token"))
	if err != nil {
		return nil, nil, err
	}
	apiInfo := api.NewAPIInfo(string(apiUrl), string(token))
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	return clientapi.NewIMarketClientRPC(cctx.Context, addr, apiInfo.AuthHeader())
}

func NewFullNode(cctx *cli.Context, legacyRepo string) (v1api.FullNode, jsonrpc.ClientCloser, error) {
	repoPath, err := GetRepoPath(cctx, "repo", legacyRepo)
	if err != nil {
		return nil, nil, err
	}
	cfgPath := path.Join(repoPath, "config.toml")
	marketCfg := config.DefaultMarketConfig
	err = config.LoadConfig(cfgPath, marketCfg)
	if err != nil {
		return nil, nil, err
	}
	nodeCfg := marketCfg.GetNode()
	apiInfo := api.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}

	return v1api.NewFullNodeRPC(cctx.Context, addr, apiInfo.AuthHeader())
}

func getMarketClientConfig(cctx *cli.Context, legacyRepo string) (*config.MarketClientConfig, error) {
	repoPath, err := GetRepoPath(cctx, "repo", legacyRepo)
	if err != nil {
		return nil, err
	}
	cfgPath := path.Join(repoPath, "config.toml")
	marketClientCfg := config.DefaultMarketClientConfig
	err = config.LoadConfig(cfgPath, marketClientCfg)
	if err != nil {
		return nil, err
	}

	return marketClientCfg, nil
}

func NewHost(cctx *cli.Context, legacyRepo string) (host.Host, error) {
	cfg, err := getMarketClientConfig(cctx, legacyRepo)
	if err != nil {
		return nil, err
	}

	if len(cfg.Libp2p.PrivateKey) == 0 {
		return nil, fmt.Errorf("private key is nil")
	}

	decodePriv, err := hex.DecodeString(cfg.Libp2p.PrivateKey)
	if err != nil {
		return nil, err
	}
	peerkey, err := crypto2.UnmarshalPrivateKey(decodePriv)
	if err != nil {
		return nil, err
	}

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Identity(peerkey),
	)
	if err != nil {
		return nil, err
	}

	return h, nil
}

func GetSignerFromRepo(cctx *cli.Context, legacyRepo string) (signer.ISigner, jsonrpc.ClientCloser, error) {
	cfg, err := getMarketClientConfig(cctx, legacyRepo)
	if err != nil {
		return nil, nil, err
	}

	return signer.NewISignerClient(false, nil)(cctx.Context, &cfg.Signer)
}

func GetAddressInfo(ctx context.Context, fapi v1api.FullNode, miner address.Address) (*peer.AddrInfo, error) {
	minerInfo, err := fapi.StateMinerInfo(ctx, miner, shared.EmptyTSK)
	if err != nil {
		return nil, err
	}
	addrs, err := utils.ConvertMultiaddr(minerInfo.Multiaddrs)
	if err != nil {
		return nil, err
	}

	return &peer.AddrInfo{ID: *minerInfo.PeerId, Addrs: addrs}, nil
}

func AddressFromContextOrDefault(cctx *cli.Context, api clientapi.IMarketClient) (address.Address, error) {
	if from := cctx.String("from"); from != "" {
		addr, err := address.NewFromString(from)
		if err != nil {
			return address.Undef, fmt.Errorf("failed to parse 'from' address: %w", err)
		}
		return addr, nil
	}

	return api.DefaultAddress(cctx.Context)
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

func IncorrectNumArgs(cctx *cli.Context) error {
	return ShowHelp(cctx, fmt.Errorf("incorrect number of arguments, got %d", cctx.NArg()))
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

	_, _ = fmt.Fprint(out, "Sending Channels\n\n")
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

	_, _ = fmt.Fprint(out, "\nReceiving Channels\n\n")
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
	_, _ = fmt.Fprint(a.app.Writer, args...)
}

func (a *AppFmt) Println(args ...interface{}) {
	_, _ = fmt.Fprintln(a.app.Writer, args...)
}

func (a *AppFmt) Printf(fmtstr string, args ...interface{}) {
	_, _ = fmt.Fprintf(a.app.Writer, fmtstr, args...)
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

func shouldAddress(s string, checkEmpty bool, allowActor bool) (address.Address, error) {
	if checkEmpty && s == "" {
		return address.Undef, fmt.Errorf("empty address string")
	}

	if allowActor {
		id, err := strconv.ParseUint(s, 10, 64)
		if err == nil {
			return address.NewIDAddress(id)
		}
	}

	return address.NewFromString(s)
}

type result struct {
	// lotus-miner query deals result
	Result []*types.MinerDeal `json:"result"`

	// boost query deals result
	BoostResult struct {
		Deals struct {
			TotalCount int         `json:"totalCount"`
			Deals      []boostDeal `json:"deals"`
		} `json:"deals"`
	} `json:"data"`
}

type bigInt big.Int

func (bi *bigInt) UnmarshalJSON(data []byte) error {
	type internal struct {
		N string `json:"n"`
	}

	var t internal
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}

	i := big.NewInt(0)
	if t.N != "0" {
		n, err := strconv.ParseUint(t.N, 10, 64)
		if err != nil {
			return err
		}
		i = big.NewIntUnsigned(n)
	}
	*bi = bigInt(i)

	return nil
}

type transfer struct {
	Type string
	Size bigInt
}

type sector struct {
	ID     bigInt
	Offset bigInt
	Length bigInt
}

type boostDeal struct {
	ID                   string
	ClientAddress        string
	ProviderAddress      string
	CreatedAt            string
	PieceCid             string
	PieceSize            bigInt
	IsVerified           bool
	ProposalLabel        string
	ProviderCollateral   bigInt
	ClientCollateral     bigInt
	StoragePricePerEpoch bigInt
	StartEpoch           bigInt
	EndEpoch             bigInt
	ClientPeerID         string
	DealDataRoot         string
	SignedProposalCid    string
	InboundFilePath      string
	ChainDealID          bigInt
	PublishCid           string
	IsOffline            bool
	Transfer             transfer
	Checkpoint           string
	Err                  string
	Sector               sector
	Message              string
}

func (d *boostDeal) minerDeal() (deal *types.MinerDeal, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	label, err := shared.NewLabelFromString(d.ProposalLabel)
	if err != nil {
		return nil, err
	}
	clientPeerID, err := peer.Decode(d.ClientPeerID)
	if err != nil {
		return nil, err
	}
	createAt, err := time.Parse(time.RFC3339, d.CreatedAt)
	if err != nil {
		return nil, err
	}
	pieceCID := shared.MustParseCid(d.PieceCid)

	id, err := uuid.Parse(d.ID)
	if err != nil {
		return nil, err
	}

	deal = &types.MinerDeal{
		ID: id,
		ClientDealProposal: market.ClientDealProposal{
			Proposal: market.DealProposal{
				PieceCID:             pieceCID,
				PieceSize:            abi.PaddedPieceSize(d.PieceSize.Int64()),
				VerifiedDeal:         d.IsVerified,
				Client:               shared.MustParseAddress(d.ClientAddress),
				Provider:             shared.MustParseAddress(d.ProviderAddress),
				Label:                label,
				StartEpoch:           abi.ChainEpoch(d.StartEpoch.Int64()),
				EndEpoch:             abi.ChainEpoch(d.EndEpoch.Int64()),
				StoragePricePerEpoch: big.Int(d.StoragePricePerEpoch),
				ProviderCollateral:   big.Int(d.ProviderCollateral),
				ClientCollateral:     big.Int(d.ClientCollateral),
			},
			// todo: query response not include Signature
			ClientSignature: crypto.Signature{},
		},
		ProposalCid:   shared.MustParseCid(d.SignedProposalCid),
		Client:        clientPeerID,
		PiecePath:     "",
		PayloadSize:   uint64(d.Transfer.Size.Int64()),
		MetadataPath:  "",
		FastRetrieval: true,
		Message:       d.Err,
		Ref: &storagemarket.DataRef{
			TransferType: "import",
			Root:         shared.MustParseCid(d.DealDataRoot),
			PieceCid:     &pieceCID,
			PieceSize:    abi.UnpaddedPieceSize(d.PieceSize.Int64()),
		},
		AvailableForRetrieval: false,
		DealID:                abi.DealID(d.ChainDealID.Int64()),
		CreationTime:          cbg.CborTime(createAt),
		SectorNumber:          abi.SectorNumber(d.Sector.ID.Int64()),
		Offset:                abi.PaddedPieceSize(d.Sector.Offset.Int64()),
		TimeStamp: types.TimeStamp{
			CreatedAt: uint64(createAt.Unix()),
			UpdatedAt: uint64(time.Now().Unix()),
		},
		SlashEpoch: -1,
		// AddFundsCid: ,
		// FundsReserved: ,
		// TransferChannelID: ,
	}
	if d.IsOffline {
		deal.Ref.TransferType = storagemarket.TTManual
	}

	if len(d.PublishCid) != 0 {
		publicCID := shared.MustParseCid(d.PublishCid)
		deal.PublishCid = &publicCID
	}
	deal.PieceStatus = types.Undefine

	switch d.Checkpoint {
	// https://github.com/filecoin-project/boost/blob/main/gql/resolver.go#L546
	case "Accepted":
		if d.Message == "Awaiting Offline Data Import" {
			deal.State = storagemarket.StorageDealWaitingForData
		}
	// https://github.com/filecoin-project/boost/blob/main/gql/resolver.go#L583
	case "IndexedAndAnnounced":
		if strings.Contains(d.Message, string(types.Proving)) {
			deal.State = storagemarket.StorageDealActive
			deal.PieceStatus = types.Proving
		}
	}

	return deal, nil
}

func getMinerPeerFunc(ctx context.Context, fapi v1api.FullNode) func(miner address.Address) peer.ID {
	minersPeer := make(map[address.Address]peer.ID)

	return func(miner address.Address) peer.ID {
		id, ok := minersPeer[miner]
		if !ok {
			minerInfo, err := fapi.StateMinerInfo(ctx, miner, shared.EmptyTSK)
			if err == nil && minerInfo.PeerId != nil {
				id = *minerInfo.PeerId
				minersPeer[miner] = *minerInfo.PeerId
			}
		}
		return id
	}
}

func getPayloadSizeFunc(dirs []string) func(pieceCID cid.Cid) uint64 {
	sizes := make(map[cid.Cid]uint64)

	return func(pieceCID cid.Cid) uint64 {
		piece := pieceCID.String()
		for _, dir := range dirs {
			fi, err := os.Stat(filepath.Join(dir, piece))
			if err == nil {
				sizes[pieceCID] = uint64(fi.Size())
			}
		}

		return sizes[pieceCID]
	}
}

// todo: remove legacy repo path after v1.13
func GetRepoPath(cctx *cli.Context, repoFlagName, oldRepoPath string) (string, error) {
	repoPath, err := homedir.Expand(cctx.String(repoFlagName))
	if err != nil {
		return "", err
	}
	has, err := exist(repoPath)
	if err != nil {
		return "", err
	}
	if !has {
		oldRepoPath, err = homedir.Expand(oldRepoPath)
		if err != nil {
			return "", err
		}
		has, err = exist(oldRepoPath)
		if err != nil {
			return "", err
		}
		if has {
			return oldRepoPath, nil
		}
	}

	return repoPath, nil
}

func exist(path string) (bool, error) {
	f, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !f.IsDir() {
		return false, fmt.Errorf("%s not a file directory", path)
	}

	return true, nil
}
