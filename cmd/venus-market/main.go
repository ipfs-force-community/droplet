package main

import (
	"log"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	cli2 "github.com/filecoin-project/venus-market/v2/cli"
	_ "github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/version"

	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
)

var mainLog = logging.Logger("main")

// Invokes are called in the order they are defined.
// nolint:golint
var (
	ExtractApiKey = builder.NextInvoke()
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
		Name:   "signer-type",
		Usage:  "signer service type(lotusnode, wallet, gateway)",
		Hidden: false,
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
		Name:  "gateway-url",
		Usage: "used to connect gateway service for sign",
	}
	GatewayTokenFlag = &cli.StringFlag{
		Name:  "gateway-token",
		Usage: "used to connect gateway service for sign",
	}

	MysqlDsnFlag = &cli.StringFlag{
		Name:  "mysql-dsn",
		Usage: "mysql connection string",
	}

	RetrievalPaymentAddress = &cli.StringFlag{
		Name:  "payment-addr",
		Usage: "payment address for retrieval, eg. f01000",
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
			cli2.StatsCmds,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
