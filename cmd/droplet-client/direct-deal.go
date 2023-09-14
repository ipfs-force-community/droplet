package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	datacap2 "github.com/filecoin-project/go-state-types/builtin/v9/datacap"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/datacap"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/ipfs-force-community/droplet/v2/cli/tablewriter"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
)

var directDealCommands = &cli.Command{
	Name:  "direct-deal",
	Usage: "direct deal tools",
	Subcommands: []*cli.Command{
		directDealAllocate,
	},
}

var directDealAllocate = &cli.Command{
	Name:  "allocate",
	Usage: "Create new allocation[s] for verified deals",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:     "miner",
			Usage:    "storage provider address[es]",
			Required: true,
			Aliases:  []string{"m"},
		},
		&cli.StringSliceFlag{
			Name:  "piece-info",
			Usage: "data pieceInfo[s] to create the allocation. The format must be pieceCid1=pieceSize1 pieceCid2=pieceSize2",
		},
		&cli.StringFlag{
			Name:  "wallet",
			Usage: "the wallet address that will used create the allocation",
		},
		&cli.BoolFlag{
			Name:  "quiet",
			Usage: "do not print the allocation list",
			Value: false,
		},
		&cli.Int64Flag{
			Name: "term-min",
			Usage: "The minimum duration which the provider must commit to storing the piece to avoid early-termination penalties (epochs).\n" +
				"Default is 180 days.",
			Value: types.MinimumVerifiedAllocationTerm,
		},
		&cli.Int64Flag{
			Name: "term-max",
			Usage: "The maximum period for which a provider can earn quality-adjusted power for the piece (epochs).\n" +
				"Default is 5 years.",
			Value: types.MaximumVerifiedAllocationTerm,
		},
		&cli.Int64Flag{
			Name: "expiration",
			Usage: "The latest epoch by which a provider must commit data before the allocation expires (epochs).\n" +
				"Default is 60 days.",
		},
	},
	Before: func(cctx *cli.Context) error {
		if !cctx.IsSet("expiration") {
			fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
			if err != nil {
				return err
			}
			defer fcloser()
			head, err := fapi.ChainHead(cctx.Context)
			if err != nil {
				return err
			}
			val := types.MaximumVerifiedAllocationExpiration + head.Height()

			return cctx.Set("expiration", strconv.FormatInt(int64(val), 10))
		}

		return nil
	},
	Action: func(cctx *cli.Context) error {
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

		// Get wallet address from input
		walletAddr, err := getProvidedOrDefaultWallet(ctx, api, cctx.String("wallet"))
		if err != nil {
			return err
		}

		// Get all minerIDs from input
		maddrs := make(map[abi.ActorID]types.MinerInfo)
		minerIds := cctx.StringSlice("miner")
		for _, id := range minerIds {
			maddr, err := address.NewFromString(id)
			if err != nil {
				return err
			}

			// Verify that minerID exists
			m, err := fapi.StateMinerInfo(ctx, maddr, types.EmptyTSK)
			if err != nil {
				return err
			}

			mid, err := address.IDFromAddress(maddr)
			if err != nil {
				return err
			}

			maddrs[abi.ActorID(mid)] = m
		}

		// Get all pieceCIDs from input
		rDataCap := types.NewInt(0)
		var pieceInfos []*abi.PieceInfo
		pieces := cctx.StringSlice("piece-info")
		for _, p := range pieces {
			pieceDetail := strings.Split(p, "=")
			if len(pieceDetail) > 2 {
				return fmt.Errorf("incorrect pieceInfo format: %s", pieceDetail)
			}

			n, err := strconv.ParseUint(pieceDetail[1], 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse the piece size for %s for pieceCid %s: %w", pieceDetail[0], pieceDetail[1], err)
			}
			pcid, err := cid.Parse(pieceDetail[0])
			if err != nil {
				return fmt.Errorf("failed to parse the pieceCid for %s: %w", pieceDetail[0], err)
			}

			pieceInfos = append(pieceInfos, &abi.PieceInfo{
				Size:     abi.PaddedPieceSize(n),
				PieceCID: pcid,
			})
			rDataCap.Add(types.NewInt(n).Int, rDataCap.Int)
		}

		// Get datacap balance
		aDataCap, err := fapi.StateVerifiedClientStatus(ctx, walletAddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		// Check that we have enough data cap to make the allocation
		if rDataCap.GreaterThan(types.NewInt(uint64(aDataCap.Int64()))) {
			return fmt.Errorf("requested datacap(%v) is greater then the available datacap(%v)", rDataCap, aDataCap)
		}

		head, err := fapi.ChainHead(ctx)
		if err != nil {
			return err
		}

		termMax := cctx.Int64("term-max")
		termMin := cctx.Int64("term-min")
		expiration := cctx.Int64("expiration")
		if termMax < termMin {
			return fmt.Errorf("maximum duration %d cannot be smaller than minimum duration %d", termMax, termMin)
		}
		if expiration < int64(head.Height()) {
			return fmt.Errorf("expiration %d smaller than current epoch %d", expiration, head.Height())
		}

		// Create allocation requests
		var allocationRequests []types.AllocationRequest
		for mid, minerInfo := range maddrs {
			for _, p := range pieceInfos {
				if uint64(minerInfo.SectorSize) < uint64(p.Size) {
					return fmt.Errorf("specified piece size %d is bigger than miner's sector size %s", uint64(p.Size), minerInfo.SectorSize.String())
				}
				allocationRequests = append(allocationRequests, types.AllocationRequest{
					Provider:   mid,
					Data:       p.PieceCID,
					Size:       p.Size,
					TermMin:    abi.ChainEpoch(termMin),
					TermMax:    abi.ChainEpoch(termMax),
					Expiration: abi.ChainEpoch(expiration),
				})
			}
		}

		reqs := &types.AllocationRequests{
			Allocations: allocationRequests,
		}
		receiverParams, err := actors.SerializeParams(reqs)
		if err != nil {
			return fmt.Errorf("failed to seralize the parameters: %w", err)
		}
		transferParams, err := actors.SerializeParams(&datacap2.TransferParams{
			To:           builtin.VerifiedRegistryActorAddr,
			Amount:       big.Mul(rDataCap, builtin.TokenPrecision),
			OperatorData: receiverParams,
		})
		if err != nil {
			return fmt.Errorf("failed to serialize transfer parameters: %w", err)
		}

		msg := &types.Message{
			To:     builtin.DatacapActorAddr,
			From:   walletAddr,
			Method: datacap.Methods.TransferExported,
			Params: transferParams,
			Value:  big.Zero(),
		}
		msgCid, err := api.MessagerPushMessage(ctx, msg, &types.MessageSendSpec{})
		if err != nil {
			return fmt.Errorf("push message failed: %v", err)
		}

		fmt.Println("submitted data cap allocation message:", msgCid.String())
		fmt.Println("waiting for message to be included in a block")

		res, err := api.MessagerWaitMessage(ctx, msgCid)
		if err != nil {
			return fmt.Errorf("waiting for message to be included in a block: %w", err)
		}
		if !res.Receipt.ExitCode.IsSuccess() {
			return fmt.Errorf("failed to execute the message with error: %s", res.Receipt.ExitCode.Error())
		}

		if !cctx.Bool("quiet") {
			return showAllocations(ctx, fapi, walletAddr, cctx.Bool("json"))
		}

		return nil
	},
}

func showAllocations(ctx context.Context, fapi v1.FullNode, walletAddr address.Address, useJSON bool) error {
	oldallocations, err := fapi.StateGetAllocations(ctx, walletAddr, types.EmptyTSK)
	if err != nil {
		return fmt.Errorf("failed to get allocations: %w", err)
	}

	newallocations, err := fapi.StateGetAllocations(ctx, walletAddr, types.EmptyTSK)
	if err != nil {
		return fmt.Errorf("failed to get allocations: %w", err)
	}

	// Map Keys. Corresponds to the standard tablewriter output
	allocationID := "AllocationID"
	client := "Client"
	provider := "Miner"
	pieceCid := "PieceCid"
	pieceSize := "PieceSize"
	tMin := "TermMin"
	tMax := "TermMax"
	expr := "Expiration"

	// One-to-one mapping between tablewriter keys and JSON keys
	tableKeysToJsonKeys := map[string]string{
		allocationID: strings.ToLower(allocationID),
		client:       strings.ToLower(client),
		provider:     strings.ToLower(provider),
		pieceCid:     strings.ToLower(pieceCid),
		pieceSize:    strings.ToLower(pieceSize),
		tMin:         strings.ToLower(tMin),
		tMax:         strings.ToLower(tMax),
		expr:         strings.ToLower(expr),
	}

	var allocs []map[string]interface{}
	for key, val := range newallocations {
		_, ok := oldallocations[key]
		if !ok {
			alloc := map[string]interface{}{
				allocationID: key,
				client:       val.Client,
				provider:     val.Provider,
				pieceCid:     val.Data,
				pieceSize:    val.Size,
				tMin:         val.TermMin,
				tMax:         val.TermMax,
				expr:         val.Expiration,
			}
			allocs = append(allocs, alloc)
		}
	}

	if useJSON {
		var jsonAllocs []map[string]interface{}
		for _, alloc := range allocs {
			jsonAlloc := make(map[string]interface{})
			for k, v := range alloc {
				jsonAlloc[tableKeysToJsonKeys[k]] = v
			}
			jsonAllocs = append(jsonAllocs, jsonAlloc)
		}

		return printJson(jsonAllocs)
	}
	tw := tablewriter.New(
		tablewriter.Col(allocationID),
		tablewriter.Col(client),
		tablewriter.Col(provider),
		tablewriter.Col(pieceCid),
		tablewriter.Col(pieceSize),
		tablewriter.Col(tMin),
		tablewriter.Col(tMax),
		tablewriter.NewLineCol(expr),
	)

	for _, alloc := range allocs {
		tw.Write(alloc)
	}

	return tw.Flush(os.Stdout)
}
