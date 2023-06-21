package main

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/ipfs-force-community/venus-common-utils/builder"

	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	_ "github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/version"

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
		EnvVars: []string{"DROPLET_PATH", "VENUS_MARKET_PATH"},
		Value:   cli2.DefMarketRepoPath,
	}

	APIListenFlag = &cli.StringFlag{
		Name:  "listen",
		Usage: "specify endpoint for listen",
		Value: "/ip4/127.0.0.1/tcp/41235",
	}

	NodeUrlFlag = &cli.StringFlag{
		Name:  "node-url",
		Usage: "url to connect to daemon service",
	}

	AuthUrlFlag = &cli.StringFlag{
		Name:  "auth-url",
		Usage: "url to connect to auth service",
	}

	MessagerUrlFlag = &cli.StringFlag{
		Name:  "messager-url",
		Usage: "url to connect messager service",
	}

	GatewayUrlFlag = &cli.StringFlag{
		Name:  "gateway-url",
		Usage: "used to connect gateway service for sign",
	}

	ChainServiceTokenFlag = &cli.StringFlag{
		Name:  "cs-token",
		Usage: "chain service token",
	}

	SignerTypeFlag = &cli.StringFlag{
		Name:  "signer-type",
		Usage: "signer service type(lotusnode, wallet, gateway)",
	}
	SignerUrlFlag = &cli.StringFlag{
		Name:  "signer-url",
		Usage: "used to connect signer service for sign",
	}
	SignerTokenFlag = &cli.StringFlag{
		Name:  "signer-token",
		Usage: "auth token for connect signer service",
	}

	MysqlDsnFlag = &cli.StringFlag{
		Name:  "mysql-dsn",
		Usage: "mysql connection string",
	}
)

func main() {
	app := &cli.App{
		Name:                 "droplet",
		Usage:                "droplet",
		Version:              version.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			RepoFlag,
		},
		Commands: []*cli.Command{
			runCmd,
			cli2.PiecesCmd,
			cli2.RetrievalCmds,
			cli2.StorageCmds,
			cli2.ActorCmd,
			cli2.NetCmd,
			cli2.DataTransfersCmd,
			cli2.DagstoreCmd,
			cli2.PieceStorageCmd,
			cli2.MarketCmds,
			cli2.StatsCmds,
		},
	}

	if err := app.Run(os.Args); err != nil {
		mainLog.Fatal(err)
	}
}
