package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
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

var minerFlag = &cli.StringSliceFlag{
	Name:     "miner",
	Usage:    "storage provider address[es]",
	Required: true,
	Aliases:  []string{"m"},
}
var walletFlag = &cli.StringFlag{
	Name:  "wallet",
	Usage: "the wallet address that will used create the allocation",
}
var termMinFlag = &cli.Int64Flag{
	Name: "term-min",
	Usage: "The minimum duration which the provider must commit to storing the piece to avoid early-termination penalties (days).\n" +
		"Default is 180 days.",
	// Value: types.MinimumVerifiedAllocationTerm,
	Value: 180,
}
var termMaxFlag = &cli.Int64Flag{ // nolint
	Name: "term-max",
	// Usage: "The maximum period for which a provider can earn quality-adjusted power for the piece (epochs).\n",
	Usage: "The maximum period for which a provider can earn quality-adjusted power for the piece (epochs).\n" +
		"Default is min(5 years, term-min + 90 days).",
	// Value: types.MaximumVerifiedAllocationTerm,
}
var expirationFlag = &cli.Int64Flag{
	Name: "expiration",
	Usage: "The latest epoch by which a provider must commit data before the allocation expires (days).\n" +
		"Default is 8 days, max is 60 days.",
	// Value: types.DefaultAllocationExpiration,
	Value: 8,
}

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
		minerFlag,
		walletFlag,
		termMinFlag,
		// termMaxFlag,
		expirationFlag,
		&cli.StringSliceFlag{
			Name:  "piece-info",
			Usage: "data pieceInfo[s] to create the allocation. The format must be pieceCid1=pieceSize1 pieceCid2=pieceSize2",
		},
		&cli.BoolFlag{
			Name:  "quiet",
			Usage: "do not print the allocation list",
			Value: false,
		},
		&cli.StringFlag{
			Name:  "manifest",
			Usage: "Path to the manifest file",
		},
		&cli.StringFlag{
			Name:  "output-allocation-to-file",
			Usage: "Output allocation information to a file.",
			Value: "allocation.txt",
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
		if cctx.IsSet("piece-info") && cctx.IsSet("manifest") {
			return fmt.Errorf("cannot specify both piece-info and manifest")
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
		var pieceInfos []*pieceInfo

		if cctx.IsSet("piece-info") {
			var dataCapCount uint64
			pieceInfos, dataCapCount, err = pieceInfosFromCtx(cctx)
			if err != nil {
				return err
			}
			rDataCap = big.NewInt(int64(dataCapCount))
		} else {
			var dataCapCount uint64
			pieceInfos, dataCapCount, err = pieceInfosFromFile(cctx)
			if err != nil {
				return err
			}
			rDataCap = big.NewInt(int64(dataCapCount))
		}

		if len(pieceInfos) == 0 {
			return fmt.Errorf("piece info is empty")
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

		allocationParams, err := getAllocationParams(cctx, head.Height())
		if err != nil {
			return err
		}

		// Create allocation requests
		var allocationRequests []types.AllocationRequest
		for mid, minerInfo := range maddrs {
			for _, p := range pieceInfos {
				if uint64(minerInfo.SectorSize) < uint64(p.pieceSize) {
					return fmt.Errorf("specified piece size %d is bigger than miner's sector size %s", uint64(p.pieceSize), minerInfo.SectorSize.String())
				}
				allocationRequests = append(allocationRequests, types.AllocationRequest{
					Provider:   mid,
					Data:       p.pieceCID,
					Size:       p.pieceSize,
					TermMin:    allocationParams.termMin,
					TermMax:    allocationParams.termMax,
					Expiration: allocationParams.expiration,
				})
			}
		}

		reqs := &types.AllocationRequests{
			Allocations: allocationRequests,
		}
		receiverParams, err := actors.SerializeParams(reqs)
		if err != nil {
			return fmt.Errorf("failed to serialize the parameters: %w", err)
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

		return showAllocations(ctx, fapi, walletAddr, cctx.Bool("json"), cctx.Bool("quiet"), cctx.String("output-allocation-to-file"), pieceInfos)
	},
}

type pieceInfo struct {
	pieceCID    cid.Cid
	pieceSize   abi.PaddedPieceSize
	payloadSize uint64
}

func pieceInfosFromCtx(cctx *cli.Context) ([]*pieceInfo, uint64, error) {
	var pieceInfos []*pieceInfo
	var rDataCap uint64
	pieces := cctx.StringSlice("piece-info")

	for _, p := range pieces {
		pieceDetail := strings.Split(p, "=")
		if len(pieceDetail) > 2 {
			return nil, 0, fmt.Errorf("incorrect pieceInfo format: %s", pieceDetail)
		}

		n, err := strconv.ParseUint(pieceDetail[1], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse the piece size for %s for pieceCid %s: %w", pieceDetail[0], pieceDetail[1], err)
		}
		if n <= 0 {
			return nil, 0, fmt.Errorf("invalid piece size %s", pieceDetail[1])
		}
		pcid, err := cid.Parse(pieceDetail[0])
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse the pieceCid for %s: %w", pieceDetail[0], err)
		}

		pieceInfos = append(pieceInfos, &pieceInfo{
			pieceSize: abi.PaddedPieceSize(n),
			pieceCID:  pcid,
		})
		rDataCap += n
	}

	return pieceInfos, rDataCap, nil
}

func pieceInfosFromFile(cctx *cli.Context) ([]*pieceInfo, uint64, error) {
	var pieceInfos []*pieceInfo
	var rDataCap uint64
	manifest := cctx.String("manifest")

	pieces, err := loadManifest(manifest)
	if err != nil {
		return nil, 0, err
	}

	for _, p := range pieces {
		if p.pieceSize <= 0 {
			return nil, 0, fmt.Errorf("invalid piece size: %d", p.pieceSize)
		}
		n := p.pieceSize.Padded()
		pieceInfos = append(pieceInfos, &pieceInfo{
			pieceSize:   n,
			pieceCID:    p.pieceCID,
			payloadSize: p.payloadSize,
		})
		if p.pieceCID == cid.Undef {
			return nil, 0, fmt.Errorf("piece cid cannot be undefined")
		}
		rDataCap += uint64(n)
	}

	return pieceInfos, rDataCap, nil
}

type allocationParams struct {
	termMin    abi.ChainEpoch
	termMax    abi.ChainEpoch
	expiration abi.ChainEpoch
}

func getAllocationParams(cctx *cli.Context, currentHeight abi.ChainEpoch) (*allocationParams, error) {
	var params allocationParams
	termMin := cctx.Int64("term-min")
	expiration := cctx.Int64("expiration")
	if termMin < 180 || termMin > 5*365 {
		return nil, fmt.Errorf("invalid term-min: %d", termMin)
	}
	params.termMin = abi.ChainEpoch(termMin) * builtin.EpochsInDay
	params.termMax = Min[abi.ChainEpoch](params.termMin+90*builtin.EpochsInDay, types.MaximumVerifiedAllocationTerm)
	if expiration <= 0 || expiration > 60 {
		return nil, fmt.Errorf("invalid expiration: %d", expiration)
	}
	params.expiration = abi.ChainEpoch(expiration)*builtin.EpochsInDay + currentHeight

	return &params, nil
}

type partAllocationInfo struct {
	AllocationID types.AllocationId
	PieceCID     cid.Cid
	Client       address.Address
}

func showAllocations(ctx context.Context, fapi v1.FullNode, walletAddr address.Address, useJSON bool, quite bool, allocationFile string, pieceInfos []*pieceInfo) error {
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
	var partAllocationInfos []partAllocationInfo
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
			client, _ := address.NewIDAddress(uint64(val.Client))
			partAllocationInfos = append(partAllocationInfos, partAllocationInfo{
				AllocationID: key,
				PieceCID:     val.Data,
				Client:       client,
			})
		}
	}

	if err := outputAllocationToFile(allocationFile, partAllocationInfos, pieceInfos); err != nil {
		fmt.Println("output allocation to file error: ", err)
	}

	if !quite {
		return nil
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

func outputAllocationToFile(allocationFile string, infos []partAllocationInfo, pieceInfo []*pieceInfo) error {
	payloadSizes := make(map[cid.Cid]uint64)
	for _, info := range pieceInfo {
		payloadSizes[info.pieceCID] = info.payloadSize
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].AllocationID < infos[j].AllocationID
	})

	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)
	if err := w.Write([]string{"AllocationID", "PieceCID", "Client", "PayloadSize"}); err != nil {
		return err
	}
	for _, info := range infos {
		if err := w.Write([]string{fmt.Sprintf("%d", info.AllocationID), info.PieceCID.String(), info.Client.String(), fmt.Sprintf("%d", payloadSizes[info.PieceCID])}); err != nil {
			return err
		}
	}
	w.Flush()

	return os.WriteFile(allocationFile, buf.Bytes(), 0644)
}
