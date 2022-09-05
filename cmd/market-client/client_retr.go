package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	cli2 "github.com/filecoin-project/venus-market/v2/cli"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/big"
)

const DefaultMaxRetrievePrice = "0"

func retrieve(ctx context.Context, cctx *cli.Context, fapi clientapi.IMarketClient, sel *client.DataSelector, printf func(string, ...interface{})) (*client.ExportRef, error) {
	var payer address.Address
	var err error
	if cctx.String("from") != "" {
		payer, err = address.NewFromString(cctx.String("from"))
	} else {
		payer, err = fapi.DefaultAddress(ctx)
	}
	if err != nil {
		return nil, err
	}

	file, err := cid.Parse(cctx.Args().Get(0))
	if err != nil {
		return nil, err
	}

	var pieceCid *cid.Cid
	if cctx.String("pieceCid") != "" {
		parsed, err := cid.Parse(cctx.String("pieceCid"))
		if err != nil {
			return nil, err
		}
		pieceCid = &parsed
	}

	var eref *client.ExportRef
	if cctx.Bool("allow-local") {
		imports, err := fapi.ClientListImports(ctx)
		if err != nil {
			return nil, err
		}

		for _, i := range imports {
			if i.Root != nil && i.Root.Equals(file) {
				eref = &client.ExportRef{
					Root:         file,
					FromLocalCAR: i.CARPath,
				}
				break
			}
		}
	}

	// no local found, so make a retrieval
	if eref == nil {
		var offer client.QueryOffer
		minerStrAddr := cctx.String("provider")
		if minerStrAddr == "" { // Local discovery
			offers, err := fapi.ClientFindData(ctx, file, pieceCid)

			var cleaned []client.QueryOffer
			// filter out offers that errored
			for _, o := range offers {
				if o.Err == "" {
					cleaned = append(cleaned, o)
				}
			}

			offers = cleaned

			// sort by price low to high
			sort.Slice(offers, func(i, j int) bool {
				return offers[i].MinPrice.LessThan(offers[j].MinPrice)
			})
			if err != nil {
				return nil, err
			}

			// TODO: parse offer strings from `client find`, make this smarter
			if len(offers) < 1 {
				fmt.Println("Failed to find file")
				return nil, nil
			}
			offer = offers[0]
		} else { // Directed retrieval
			minerAddr, err := address.NewFromString(minerStrAddr)
			if err != nil {
				return nil, err
			}
			offer, err = fapi.ClientMinerQueryOffer(ctx, minerAddr, file, pieceCid)
			if err != nil {
				return nil, err
			}
		}
		if offer.Err != "" {
			return nil, fmt.Errorf("offer error: %s", offer.Err)
		}

		maxPrice := types2.MustParseFIL(DefaultMaxRetrievePrice)

		if cctx.String("maxPrice") != "" {
			maxPrice, err = types2.ParseFIL(cctx.String("maxPrice"))
			if err != nil {
				return nil, fmt.Errorf("parsing maxPrice: %w", err)
			}
		}

		if offer.MinPrice.GreaterThan(big.Int(maxPrice)) {
			return nil, fmt.Errorf("failed to find offer satisfying maxPrice: %s", maxPrice)
		}

		o := offer.Order(payer)
		o.DataSelector = sel

		subscribeEvents, err := fapi.ClientGetRetrievalUpdates(ctx)
		if err != nil {
			return nil, fmt.Errorf("error setting up retrieval updates: %w", err)
		}
		retrievalRes, err := fapi.ClientRetrieve(ctx, o)
		if err != nil {
			return nil, fmt.Errorf("error setting up retrieval: %w", err)
		}

		start := time.Now()
	readEvents:
		for {
			var evt client.RetrievalInfo
			select {
			case <-ctx.Done():
				return nil, errors.New("retrieval timed out")
			case evt = <-subscribeEvents:
				if evt.ID != retrievalRes.DealID {
					// we can't check the deal ID ahead of time because:
					// 1. We need to subscribe before retrieving.
					// 2. We won't know the deal ID until after retrieving.
					continue
				}
			}

			event := "New"
			if evt.Event != nil {
				event = retrievalmarket.ClientEvents[*evt.Event]
			}

			printf("Recv %s, Paid %s, %s (%s), %s\n",
				types2.SizeStr(types2.NewInt(evt.BytesReceived)),
				types2.FIL(evt.TotalPaid),
				strings.TrimPrefix(event, "ClientEvent"),
				strings.TrimPrefix(retrievalmarket.DealStatuses[evt.Status], "DealStatus"),
				time.Since(start).Truncate(time.Millisecond),
			)

			switch evt.Status {
			case retrievalmarket.DealStatusCompleted:
				break readEvents
			case retrievalmarket.DealStatusRejected:
				return nil, fmt.Errorf("retrieval Proposal Rejected: %s", evt.Message)
			case retrievalmarket.DealStatusCancelled,
				retrievalmarket.DealStatusDealNotFound,
				retrievalmarket.DealStatusErrored:
				return nil, fmt.Errorf("retrieval Error: %s", evt.Message)
			}
		}

		eref = &client.ExportRef{
			Root:   file,
			DealID: retrievalRes.DealID,
		}
	}

	return eref, nil
}

var retrFlagsCommon = []cli.Flag{
	&cli.StringFlag{
		Name:  "from",
		Usage: "address to send transactions from",
	},
	&cli.StringFlag{
		Name:    "provider",
		Usage:   "provider to use for retrieval, if not present it'll use local discovery",
		Aliases: []string{"miner"},
	},
	&cli.StringFlag{
		Name:  "maxPrice",
		Usage: fmt.Sprintf("maximum price the client is willing to consider (default: %s FIL)", DefaultMaxRetrievePrice),
	},
	&cli.StringFlag{
		Name:  "pieceCid",
		Usage: "require data to be retrieved from a specific Piece CID",
	},
	&cli.BoolFlag{
		Name: "allow-local",
		// todo: default to true?
	},
}

var clientRetrieveCmd = &cli.Command{
	Name:      "retrieve",
	Usage:     "Retrieve data from network",
	ArgsUsage: "[dataCid outputPath]",
	Description: `Retrieve data from the Filecoin network.

The retrieve command will attempt to find a provider make a retrieval deal with
them. In case a provider can't be found, it can be specified with the --provider
flag.

By default the data will be interpreted as DAG-PB UnixFSv1 File. Alternatively
a CAR file containing the raw IPLD graph can be exported by setting the --car
flag.

Partial Retrieval:

The --data-selector flag can be used to specify a sub-graph to fetch. The
selector can be specified as either IPLD datamodel text-path selector, or IPLD
json selector.

In case of unixfs retrieval, the selector must point at a single root node, and
match the entire graph under that node.

In case of CAR retrieval, the selector must have one common "sub-root" node.

Examples:

- Retrieve a file by CID
	# market-client retrieval retrieve --maxPrice 0.1fil bafyk... my-file

- Retrieve a file by CID from f0123
	# market-client retrieval retrieve --maxPrice 0.1fil --miner f0123 bafyk... my-file

- Retrieve a first file from a specified directory
	$ market-client retrieval retrieve --data-selector /Links/0/Hash bafyk... my-file.txt
`,
	Flags: append([]cli.Flag{
		&cli.BoolFlag{
			Name:  "car",
			Usage: "Export to a car file instead of a regular file",
		},
		&cli.StringFlag{
			Name:    "data-selector",
			Aliases: []string{"datamodel-path-selector"},
			Usage:   "IPLD datamodel text-path selector, or IPLD json selector",
		},
		&cli.BoolFlag{
			Name:  "car-export-merkle-proof",
			Usage: "(requires --data-selector and --car) Export data-selector merkle proof",
		},
	}, retrFlagsCommon...),
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return cli2.ShowHelp(cctx, fmt.Errorf("incorrect number of arguments"))
		}

		if cctx.Bool("car-export-merkle-proof") {
			if !cctx.Bool("car") || !cctx.IsSet("data-selector") {
				return cli2.ShowHelp(cctx, fmt.Errorf("--car-export-merkle-proof requires --car and --data-selector"))
			}
		}

		fapi, closer, err := cli2.NewMarketClientNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cctx)
		afmt := cli2.NewAppFmt(cctx.App)

		var s *client.DataSelector
		if sel := client.DataSelector(cctx.String("data-selector")); sel != "" {
			s = &sel
		}

		eref, err := retrieve(ctx, cctx, fapi, s, afmt.Printf)
		if err != nil {
			return err
		}

		if s != nil {
			eref.DAGs = append(eref.DAGs, client.DagSpec{DataSelector: s, ExportMerkleProof: cctx.Bool("car-export-merkle-proof")})
		}

		err = fapi.ClientExport(ctx, *eref, client.FileRef{
			Path:  cctx.Args().Get(1),
			IsCAR: cctx.Bool("car"),
		})
		if err != nil {
			return err
		}
		afmt.Println("Success")
		return nil
	},
}

type bytesReaderAt struct {
	btr *bytes.Reader
}

func (b bytesReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	return b.btr.ReadAt(p, off)
}

var _ io.ReaderAt = &bytesReaderAt{}
