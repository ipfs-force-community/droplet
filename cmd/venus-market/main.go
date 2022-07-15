package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/filecoin-project/venus-market/v2/cmd"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	cli2 "github.com/filecoin-project/venus-market/v2/cli"
	"github.com/filecoin-project/venus-market/v2/config"
	_ "github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/version"

	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
)

var mainLog = logging.Logger("main")

// Invokes are called in the order they are defined.
// nolint:golint
var (
	InitJournalKey = builder.NextInvoke() //nolint
	ExtractApiKey  = builder.NextInvoke()
)

var (
	RepoFlag = &cli.StringFlag{
		Name:    "repo",
		EnvVars: []string{"VENUS_MARKET_PATH"},
		Value:   "~/.venusmarket",
	}

	NodeUrlFlag = &cli.StringFlag{
		Name:  "node-url",
		Usage: "url to connect to daemon service",
	}
	NodeTokenFlag = &cli.StringFlag{
		Name:  "node-token",
		Usage: "node token",
	}

	AuthUrlFlag = &cli.StringFlag{
		Name:  "auth-url",
		Usage: "url to connect to auth service",
	}
	AuthTokeFlag = &cli.StringFlag{
		Name:  "auth-token",
		Usage: "token for connect venus components",
	}

	MessagerUrlFlag = &cli.StringFlag{
		Name:  "messager-url",
		Usage: "url to connect messager service",
	}
	MessagerTokenFlag = &cli.StringFlag{
		Name:   "messager-token",
		Usage:  "messager token",
		Hidden: true,
	}

	SignerTypeFlag = &cli.StringFlag{
		Name:        "signer-type",
		Usage:       "signer service type（wallet, gateway）",
		DefaultText: "wallet",
	}
	HidenSignerTypeFlag = &cli.StringFlag{
		Name:        "signer-type",
		Usage:       "signer service type（wallet, gateway）",
		DefaultText: "wallet",
		Hidden:      true,
	}

	SignerUrlFlag = &cli.StringFlag{
		Name:  "signer-url",
		Usage: "used to connect signer service for sign",
	}
	SignerTokenFlag = &cli.StringFlag{
		Name:  "signer-token",
		Usage: "auth token for connect signer service",
	}

	GatewayUrlFlag = &cli.StringFlag{
		Name:    "gateway-url",
		Aliases: []string{"signer-url"},
		Usage:   "used to connect gateway service for sign",
	}
	GatewayTokenFlag = &cli.StringFlag{
		Name:    "gateway-token",
		Aliases: []string{"signer-token"},
		Usage:   "used to connect gateway service for sign",
	}

	WalletUrlFlag = &cli.StringFlag{
		Name:    "wallet-url",
		Aliases: []string{"signer-url"},
		Usage:   "used to connect signer wallet for sign",
	}
	WalletTokenFlag = &cli.StringFlag{
		Name:    "wallet-token",
		Aliases: []string{"signer-token"},
		Usage:   "auth token for connect wallet service",
	}

	MysqlDsnFlag = &cli.StringFlag{
		Name:  "mysql-dsn",
		Usage: "mysql connection string",
	}

	MinerListFlag = &cli.StringSliceFlag{
		Name:  "miner",
		Usage: "support miner(f01000:jimmy)",
	}
	PaymentAddressFlag = &cli.StringFlag{
		Name:  "payment-addr",
		Usage: "payment address for receive retrieval address",
	}
)

func main() {
	app := &cli.App{
		Name:                 "venus-market",
		Usage:                "venus-market",
		Version:              version.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			RepoFlag,
		},
		Commands: []*cli.Command{
			soloRunCmd,
			poolRunCmd,
			cli2.PiecesCmd,
			cli2.RetrievalDealsCmd,
			cli2.StorageDealsCmd,
			cli2.ActorCmd,
			cli2.NetCmd,
			cli2.DataTransfersCmd,
			cli2.DagstoreCmd,
			cli2.MigrateCmd,
			cli2.PieceStorageCmd,
			cli2.MarketCmds,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func prepare(cctx *cli.Context, defSignerType config.SignerType) (*config.MarketConfig, error) {
	if !cctx.IsSet(HidenSignerTypeFlag.Name) {
		if err := cctx.Set(HidenSignerTypeFlag.Name, defSignerType); err != nil {
			return nil, fmt.Errorf("set %s with wallet failed %v", HidenSignerTypeFlag.Name, err)
		}
	}
	cfg := config.DefaultMarketConfig
	cfg.HomeDir = cctx.String(RepoFlag.Name)
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}
	mainLog.Info("load config from path ", cfgPath)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//create
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag %w", err)
		}

		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("save config to %s %w", cfgPath, err)
		}
	} else if err == nil {
		//loadConfig
		err = config.LoadConfig(cfgPath, cfg)
		if err != nil {
			return nil, err
		}

		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag %w", err)
		}
	} else {
		return nil, err
	}
	return cfg, cmd.FetchAndLoadBundles(cctx.Context, cfg.Node)
}

func flagData(cctx *cli.Context, cfg *config.MarketConfig) error {
	if cctx.IsSet(NodeUrlFlag.Name) {
		cfg.Node.Url = cctx.String(NodeUrlFlag.Name)
	}

	if cctx.IsSet(MessagerUrlFlag.Name) {
		cfg.Messager.Url = cctx.String(MessagerUrlFlag.Name)
	}

	if cctx.IsSet(AuthUrlFlag.Name) {
		cfg.AuthNode.Url = cctx.String(AuthUrlFlag.Name)
	}

	if cctx.IsSet(SignerTypeFlag.Name) {
		cfg.Signer.SignerType = cctx.String(SignerTypeFlag.Name)
	}

	if cctx.IsSet(SignerUrlFlag.Name) {
		cfg.Signer.Url = cctx.String(SignerUrlFlag.Name)
	}

	if cctx.IsSet(AuthTokeFlag.Name) {
		cfg.Node.Token = cctx.String(AuthTokeFlag.Name)

		if len(cfg.AuthNode.Url) > 0 {
			cfg.AuthNode.Token = cctx.String(AuthTokeFlag.Name)
		}

		if len(cfg.Messager.Url) > 0 {
			cfg.Messager.Token = cctx.String(AuthTokeFlag.Name)
		}

		if cfg.Signer.SignerType == "gateway" {
			cfg.Signer.Token = cctx.String(AuthTokeFlag.Name)
		}
	}

	if cctx.IsSet(NodeTokenFlag.Name) {
		cfg.Node.Token = cctx.String(NodeTokenFlag.Name)
	}
	if cctx.IsSet(MessagerTokenFlag.Name) {
		cfg.Messager.Token = cctx.String(MessagerTokenFlag.Name)
	}
	if cctx.IsSet(SignerTokenFlag.Name) {
		cfg.Signer.Token = cctx.String(SignerTokenFlag.Name)
	}

	if cctx.IsSet(MysqlDsnFlag.Name) {
		cfg.Mysql.ConnectionString = cctx.String(MysqlDsnFlag.Name)
	}

	if cctx.IsSet(MinerListFlag.Name) {
		storageMiners := make(map[config.Address]struct{}, 0)
		for _, storageMiner := range cfg.StorageMiners {
			if _, ok := storageMiners[storageMiner.Addr]; !ok {
				storageMiners[storageMiner.Addr] = struct{}{}
			}
		}

		addrStrs := cctx.StringSlice(MinerListFlag.Name)
		for _, miners := range addrStrs {
			addrStr := strings.Split(miners, ":")
			addr, err := address.NewFromString(addrStr[0])
			if err != nil {
				return fmt.Errorf("flag provide a wrong address %s %w", addrStr, err)
			}
			account := ""
			if len(addrStr) >= 2 {
				account = addrStr[1]
			}

			if _, ok := storageMiners[config.Address(addr)]; !ok {
				cfg.StorageMiners = append(cfg.StorageMiners, config.User{
					Addr:    config.Address(addr),
					Account: account,
				})
			}
		}
	}

	if cctx.IsSet(PaymentAddressFlag.Name) {
		addrStr := strings.Split(cctx.String(PaymentAddressFlag.Name), ":")
		addr, err := address.NewFromString(addrStr[0])
		if err != nil {
			return fmt.Errorf("flag provide a wrong address %s %w", addrStr, err)
		}
		account := ""
		if len(addrStr) >= 2 {
			account = addrStr[1]
		}
		cfg.RetrievalPaymentAddress = config.User{
			Addr:    config.Address(addr),
			Account: account,
		}
	}
	return nil
}
