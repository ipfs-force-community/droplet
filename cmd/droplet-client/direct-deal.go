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
	verifregst "github.com/filecoin-project/go-state-types/builtin/v9/verifreg"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/datacap"
	"github.com/filecoin-project/venus/venus-shared/api"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	marketapi "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	types2 "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/google/uuid"
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
	Usage: "The minimum duration which the provider must commit to storing the piece to avoid early-termination penalties (epochs).\n" +
		"Default is 180 days.",
	Aliases: []string{"tmin"},
	Value:   verifregst.MinimumVerifiedAllocationTerm,
}
var termMaxFlag = &cli.Int64Flag{
	Name: "term-max",
	Usage: "The maximum period for which a provider can earn quality-adjusted power for the piece (epochs).\n" +
		"Default is 5 years.",
	Aliases: []string{"tmax"},
	Value:   verifregst.MaximumVerifiedAllocationTerm,
}
var expirationFlag = &cli.Int64Flag{
	Name: "expiration",
	Usage: "The latest epoch by which a provider must commit data before the allocation expires (epochs).\n" +
		"Default is 60 days.",
	Value: verifregst.MaximumVerifiedAllocationExpiration,
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
		termMaxFlag,
		expirationFlag,
		&cli.StringSliceFlag{
			Name:   "piece-info",
			Usage:  "data pieceInfo[s] to create the allocation. The format must be pieceCid1=pieceSize1 pieceCid2=pieceSize2",
			Hidden: true,
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
			Value: "allocation.csv",
		},
		&cli.StringFlag{
			Name:  "droplet-url",
			Usage: "Url of the droplet service",
		},
		&cli.StringFlag{
			Name:  "droplet-token",
			Usage: "Token of the droplet service",
		},
		&cli.IntFlag{
			Name:  "start-epoch",
			Usage: "start epoch by when the deal should be proved by provider on-chain (default: 8 days from now)",
		},
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
		var rDataCap big.Int
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
		if aDataCap == nil {
			return fmt.Errorf("datacap not found")
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

		oldAllocations, err := fapi.StateGetAllocations(ctx, walletAddr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("failed to get allocations: %w", err)
		}

		res, err := api.MessagerWaitMessage(ctx, msgCid)
		if err != nil {
			return fmt.Errorf("waiting for message to be included in a block: %w", err)
		}
		if !res.Receipt.ExitCode.IsSuccess() {
			return fmt.Errorf("failed to execute the message with error: %s", res.Receipt.ExitCode.Error())
		}

		newAllocations, err := findNewAllocations(ctx, fapi, walletAddr, oldAllocations)
		if err != nil {
			return fmt.Errorf("failed to find new allocations: %w", err)
		}

		if err := writeAllocationsToFile(cctx.String("output-allocation-to-file"), newAllocations, pieceInfos); err != nil {
			fmt.Println("failed to write allocations to file: ", err)
		}

		if err := showAllocations(newAllocations, cctx.Bool("json"), cctx.Bool("quiet")); err != nil {
			fmt.Println("failed to show allocations: ", err)
		}

		if cctx.IsSet("droplet-url") {
			fmt.Println("importing deal to droplet")
			if err := autoImportDealToDroplet(cctx, newAllocations, pieceInfos); err != nil {
				return fmt.Errorf("failed to import deal to droplet: %w", err)
			}
			fmt.Println("successfully imported deal to droplet")
		}

		return nil
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

		pieceSize := abi.UnpaddedPieceSize(n).Padded()
		pieceInfos = append(pieceInfos, &pieceInfo{
			pieceSize: pieceSize,
			pieceCID:  pcid,
		})
		rDataCap += uint64(pieceSize)
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
	termMin := cctx.Int64(termMinFlag.Name)
	termMax := cctx.Int64(termMaxFlag.Name)
	expiration := cctx.Int64(expirationFlag.Name)

	if termMax < termMin {
		return nil, fmt.Errorf("maximum duration %d cannot be smaller than minimum duration %d", termMax, termMin)
	}

	params.termMin = abi.ChainEpoch(termMin)
	params.termMax = abi.ChainEpoch(termMax)
	params.expiration = abi.ChainEpoch(expiration) + currentHeight

	return &params, nil
}

func findNewAllocations(ctx context.Context, fapi v1.FullNode, walletAddr address.Address, oldAllocations map[types.AllocationId]types.Allocation) (map[types.AllocationId]types.Allocation, error) {
	allAllocations, err := fapi.StateGetAllocations(ctx, walletAddr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to get allocations: %w", err)
	}

	newAllocations := make(map[types.AllocationId]types.Allocation, len(allAllocations)-len(oldAllocations))
	for k, v := range allAllocations {
		if _, ok := oldAllocations[k]; !ok {
			newAllocations[k] = v
		}
	}

	return newAllocations, nil
}

type partAllocationInfo struct {
	AllocationID types.AllocationId
	PieceCID     cid.Cid
	Client       address.Address
}

func writeAllocationsToFile(allocationFile string, allocations map[types.AllocationId]types.Allocation, pieceInfo []*pieceInfo) error {
	payloadSizes := make(map[cid.Cid]uint64)
	for _, info := range pieceInfo {
		payloadSizes[info.pieceCID] = info.payloadSize
	}

	infos := make([]partAllocationInfo, 0, len(allocations))
	for id, v := range allocations {
		clientAddr, _ := address.NewIDAddress(uint64(v.Client))
		infos = append(infos, partAllocationInfo{
			AllocationID: id,
			PieceCID:     v.Data,
			Client:       clientAddr,
		})
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

	fmt.Println("writing allocations to:", allocationFile)

	return os.WriteFile(allocationFile, buf.Bytes(), 0644)
}

func showAllocations(allocations map[types.AllocationId]types.Allocation, useJSON bool, quite bool) error {
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
	for key, val := range allocations {
		clientAddr, _ := address.NewIDAddress(uint64(val.Client))
		providerAddr, _ := address.NewIDAddress(uint64(val.Provider))
		alloc := map[string]interface{}{
			allocationID: key,
			client:       clientAddr,
			provider:     providerAddr,
			pieceCid:     val.Data,
			pieceSize:    val.Size,
			tMin:         val.TermMin,
			tMax:         val.TermMax,
			expr:         val.Expiration,
		}
		allocs = append(allocs, alloc)
	}

	if quite {
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
		tablewriter.Col(expr),
	)

	for _, alloc := range allocs {
		tw.Write(alloc)
	}

	return tw.Flush(os.Stdout)
}

func autoImportDealToDroplet(cliCtx *cli.Context, allocations map[types.AllocationId]types.Allocation, pieceInfos []*pieceInfo) error {
	ctx := cliCtx.Context
	dropletURL := cliCtx.String("droplet-url")
	dropletToken := cliCtx.String("droplet-token")

	apiInfo := api.NewAPIInfo(dropletURL, dropletToken)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return err
	}

	mapi, close, err := marketapi.NewIMarketRPC(ctx, addr, apiInfo.AuthHeader())
	if err != nil {
		return err
	}
	defer close()

	fapi, fclose, err := cli2.NewFullNode(cliCtx, cli2.OldClientRepoPath)
	if err != nil {
		return err
	}
	defer fclose()

	params := types2.DirectDealParams{
		SkipCommP:         true,
		SkipGenerateIndex: true,
		NoCopyCarFile:     true,
		DealParams:        make([]types2.DirectDealParam, 0, len(allocations)),
	}

	payloadSizes := make(map[cid.Cid]uint64)
	for _, info := range pieceInfos {
		payloadSizes[info.pieceCID] = info.payloadSize
	}

	startEpoch, err := cli2.GetStartEpoch(cliCtx, fapi)
	if err != nil {
		return err
	}

	for id, alloc := range allocations {
		clientAddr, _ := address.NewIDAddress(uint64(alloc.Client))
		endEpoch, err := cli2.CheckAndGetEndEpoch(ctx, fapi, clientAddr, uint64(id), startEpoch)
		if err != nil {
			return err
		}

		params.DealParams = append(params.DealParams, types2.DirectDealParam{
			DealUUID:     uuid.New(),
			AllocationID: uint64(id),
			PayloadSize:  payloadSizes[alloc.Data],
			Client:       clientAddr,
			PieceCID:     alloc.Data,
			StartEpoch:   startEpoch,
			EndEpoch:     endEpoch,
		})
	}

	return mapi.ImportDirectDeal(ctx, &params)
}
