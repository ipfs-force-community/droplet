package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"text/tabwriter"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin"
	verifregtypes "github.com/filecoin-project/go-state-types/builtin/v15/verifreg"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	"github.com/filecoin-project/venus/venus-shared/types"
	sharedutils "github.com/filecoin-project/venus/venus-shared/utils"
	msgparser "github.com/filecoin-project/venus/venus-shared/utils/msg_parser"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	"github.com/urfave/cli/v2"
)

var datacapCmds = &cli.Command{
	Name:  "datacap",
	Usage: "datacap helper commands",
	Subcommands: []*cli.Command{
		datacapExtendCmd,
		datacapClaimsListCmd,
		datacapAllocationListCmd,
	},
}

var datacapExtendCmd = &cli.Command{
	Name:  "extend",
	Usage: "extend datacap expiration",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:     "max-term",
			Usage:    "datacap max term",
			Required: true,
		},
		&cli.Uint64SliceFlag{
			Name:  "claimId",
			Usage: "claim id array",
		},
		&cli.StringFlag{
			Name:  "from",
			Usage: "address to send the message",
		},
		&cli.BoolFlag{
			Name:  "auto",
			Usage: "automatically select eligible datacap renewals",
		},
		&cli.Int64Flag{
			Name:  "expiration-cutoff",
			Usage: "when use --auto flag, skip datacap whose current expiration is more than <cutoff> epochs from now (infinity if unspecified)",
		},
		&cli.IntFlag{
			Name:  "max-claims",
			Usage: "maximum number of claims to extend (infinity if unspecified)",
			Value: 300,
		},
	},
	ArgsUsage: "<provider address>",
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.Args().Len() == 0 {
			return fmt.Errorf("must pass provider")
		}

		api, closer, err := cli2.NewMarketClientNode(cliCtx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := cli2.ReqContext(cliCtx)

		fapi, fcloser, err := cli2.NewFullNode(cliCtx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		provider, err := address.NewFromString(cliCtx.Args().First())
		if err != nil {
			return fmt.Errorf("parse provider failed: %v", err)
		}
		providerID, err := addressToActorID(provider)
		if err != nil {
			return err
		}

		var fromAddr address.Address
		if cliCtx.IsSet("from") {
			fromAddr, err = address.NewFromString(cliCtx.String("from"))
			if err != nil {
				return err
			}
		} else {
			fromAddr, err = api.DefaultAddress(ctx)
			if err != nil {
				return err
			}
		}
		idAddr, err := fapi.StateLookupID(ctx, fromAddr, types.EmptyTSK)
		if err != nil {
			return err
		}
		fromID, err := addressToActorID(idAddr)
		if err != nil {
			return err
		}

		termMax := abi.ChainEpoch(cliCtx.Int64("max-term"))
		if termMax > types.MaximumVerifiedAllocationTerm {
			return fmt.Errorf("max term %d greater than %d", termMax, types.MaximumVerifiedAllocationTerm)
		}

		head, err := fapi.ChainHead(ctx)
		if err != nil {
			return err
		}
		claims, err := fapi.StateGetClaims(ctx, provider, types.EmptyTSK)
		if err != nil {
			return err
		}

		claimTermsParams := &verifregtypes.ExtendClaimTermsParams{}
		if cliCtx.Bool("auto") {
			cutoff := abi.ChainEpoch(cliCtx.Int64("expiration-cutoff"))
			for id, claim := range claims {
				if err := checkClaim(ctx, fapi, head, provider, fromID, termMax, claim, cutoff); err != nil {
					if !errors.Is(err, errNotNeedExtend) {
						fmt.Printf("check claim %d error: %v\n", id, err)
					}
					continue
				}
				claimTermsParams.Terms = append(claimTermsParams.Terms, verifregtypes.ClaimTerm{
					Provider: providerID,
					ClaimId:  verifregtypes.ClaimId(id),
					TermMax:  termMax,
				})
			}
		} else if cliCtx.IsSet("claimId") {
			claimIds := cliCtx.Uint64Slice("claimId")
			for _, id := range claimIds {
				claim, ok := claims[types.ClaimId(id)]
				if !ok {
					continue
				}
				if err := checkClaim(ctx, fapi, head, provider, fromID, termMax, claim, -1); err != nil {
					if !errors.Is(err, errNotNeedExtend) {
						fmt.Printf("check claim %d error: %v\n", id, err)
					}
					continue
				}
				claimTermsParams.Terms = append(claimTermsParams.Terms, verifregtypes.ClaimTerm{
					Provider: providerID,
					ClaimId:  verifregtypes.ClaimId(id),
					TermMax:  termMax,
				})
			}
		} else {
			return fmt.Errorf("must pass --claimId flag or --auto flag")
		}

		if len(claimTermsParams.Terms) == 0 {
			fmt.Println("no claim need extend")
			return nil
		}

		maxClaims := cliCtx.Int("max-claims")

		var wg sync.WaitGroup
		ch := make(chan struct{}, 5)
		for len(claimTermsParams.Terms) > 0 {
			ch <- struct{}{}
			wg.Add(1)

			var claimTerms []verifregtypes.ClaimTerm
			if len(claimTermsParams.Terms) > maxClaims {
				claimTerms = claimTermsParams.Terms[:maxClaims]
				claimTermsParams.Terms = claimTermsParams.Terms[maxClaims:]
			} else {
				claimTerms = claimTermsParams.Terms
				claimTermsParams.Terms = nil
			}
			if len(claimTerms) == 0 {
				break
			}

			go func(claimTerms []verifregtypes.ClaimTerm) {
				defer func() {
					<-ch
					wg.Done()
				}()

				if err := pushAndWaitMsg(ctx, fapi, api, fromAddr, &verifregtypes.ExtendClaimTermsParams{Terms: claimTerms}); err != nil {
					fmt.Println(err)
				}
			}(claimTerms)
		}

		wg.Wait()

		return nil
	},
}

func pushAndWaitMsg(ctx context.Context,
	fapi v1.FullNode,
	api clientapi.IMarketClient,
	fromAddr address.Address,
	claimTermsParams *verifregtypes.ExtendClaimTermsParams,
) error {
	params, serializeErr := actors.SerializeParams(claimTermsParams)
	if serializeErr != nil {
		return fmt.Errorf("serialize params error: %v", serializeErr)
	}

	msg := types.Message{
		From:   fromAddr,
		To:     builtin.VerifiedRegistryActorAddr,
		Method: builtin.MethodsVerifiedRegistry.ExtendClaimTerms,
		Params: params,
	}

	msgCID, err := api.MessagerPushMessage(ctx, &msg, nil)
	if err != nil {
		return fmt.Errorf("push message error: %v", err)
	}
	fmt.Printf("wait message: %v\n", msgCID)

	msgLookup, err := api.MessagerWaitMessage(ctx, msgCID)
	if err != nil {
		return err
	}

	if msgLookup.Receipt.ExitCode.IsError() {
		return fmt.Errorf("message execute error, exit code: %v", msgLookup.Receipt.ExitCode)
	}

	if err := sharedutils.LoadBuiltinActors(ctx, fapi); err != nil {
		return err
	}
	parser, err := msgparser.NewMessageParser(fapi)
	if err != nil {
		return err
	}

	_, ret, err := parser.ParseMessage(ctx, &msg, &msgLookup.Receipt)
	if err != nil {
		return fmt.Errorf("parse message error: %v", err)
	}

	claimTermsReturn, ok := ret.(*verifregtypes.ExtendClaimTermsReturn)
	if !ok {
		return fmt.Errorf("expect type %T, actual type %T", &verifregtypes.ExtendClaimTermsReturn{}, ret)
	}

	if len(claimTermsReturn.FailCodes) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
		fmt.Fprintln(w, "\nError occurred:\nClaimID\tErrorCode")

		for _, failCode := range claimTermsReturn.FailCodes {
			fmt.Fprintf(w, "%d\t%d\n", claimTermsParams.Terms[failCode.Idx], failCode.Code)
		}

		return w.Flush()
	}

	return nil
}

var errNotNeedExtend = fmt.Errorf("not need extend")

func checkClaim(ctx context.Context,
	fapi v1.FullNode,
	head *types.TipSet,
	provider address.Address,
	fromID abi.ActorID,
	termMax abi.ChainEpoch,
	claim types.Claim,
	cutoff abi.ChainEpoch,
) error {
	if claim.Client != fromID {
		return fmt.Errorf("client %d not match form actor id %d", claim.Client, fromID)
	}

	if claim.TermMax >= termMax {
		return fmt.Errorf("new term max(%d) smaller than old term max(%d)", termMax, claim.TermMax)
	}
	expiration := claim.TermStart + claim.TermMax - head.Height()
	if expiration <= 0 {
		// already expiration
		return fmt.Errorf("claim already expiration")
	}
	// if cutoff is negative number, skip check
	if cutoff >= 0 {
		if expiration > cutoff {
			return errNotNeedExtend
		}
	}

	sectorExpiration, err := fapi.StateSectorExpiration(ctx, provider, claim.Sector, types.EmptyTSK)
	if err != nil {
		return fmt.Errorf("got sector %d expiration failed: %v", claim.Sector, err)
	} else if sectorExpiration.OnTime <= head.Height() ||
		(sectorExpiration.Early != 0 && sectorExpiration.Early <= head.Height()) {
		return fmt.Errorf("sector already expiration")
	}

	return nil
}

type claimWithID struct {
	id uint64
	types.Claim
}

var datacapClaimsListCmd = &cli.Command{
	Name:  "list-claim",
	Usage: "list claims by provider address",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "client",
			Usage: "output only claims containing client",
		},
	},
	ArgsUsage: "<provider address>",
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.Args().Len() == 0 {
			return fmt.Errorf("must pass provider address")
		}

		fapi, fcloser, err := cli2.NewFullNode(cliCtx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cliCtx)

		provider, err := address.NewFromString(cliCtx.Args().First())
		if err != nil {
			return err
		}
		claims, err := fapi.StateGetClaims(ctx, provider, types.EmptyTSK)
		if err != nil {
			return err
		}
		if len(claims) == 0 {
			return nil
		}

		client := abi.ActorID(0)
		if cliCtx.IsSet("client") {
			clientAddr, err := address.NewFromString(cliCtx.String("client"))
			if err != nil {
				return err
			}
			idAddr, err := fapi.StateLookupID(ctx, clientAddr, types.EmptyTSK)
			if err != nil {
				return err
			}
			client, err = addressToActorID(idAddr)
			if err != nil {
				return err
			}
		}

		claimWithIDs := make([]claimWithID, 0, len(claims))
		for id, claim := range claims {
			if client != 0 && client != claim.Client {
				continue
			}
			claimWithIDs = append(claimWithIDs, claimWithID{
				id:    uint64(id),
				Claim: claim,
			})

		}
		sort.Slice(claimWithIDs, func(i, j int) bool {
			return claimWithIDs[i].id < claimWithIDs[j].id
		})

		w := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ClaimID\tProvider\tClient\tExpiration\tSize\tTermMin\tTermMax\tTermStart\tSector\tData")
		for _, claim := range claimWithIDs {
			fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%v\n", claim.id, claim.Provider, claim.Client, claim.TermStart+claim.TermMax,
				claim.Size, claim.TermMin, claim.TermMax, claim.TermStart, claim.Sector, claim.Data)
		}

		return w.Flush()
	},
}

type allocationWithID struct {
	id uint64
	types.Allocation
}

var datacapAllocationListCmd = &cli.Command{
	Name:  "list-allocation",
	Usage: "list allocations by client address",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "provider",
			Usage: "output only allocations containing provider",
		},
	},
	ArgsUsage: "<client address>",
	Action: func(cliCtx *cli.Context) error {
		if cliCtx.Args().Len() == 0 {
			return fmt.Errorf("must pass client address")
		}

		fapi, fcloser, err := cli2.NewFullNode(cliCtx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		ctx := cli2.ReqContext(cliCtx)

		client, err := address.NewFromString(cliCtx.Args().First())
		if err != nil {
			return err
		}
		allocations, err := fapi.StateGetAllocations(ctx, client, types.EmptyTSK)
		if err != nil {
			return err
		}
		if len(allocations) == 0 {
			return nil
		}

		provider := abi.ActorID(0)
		if cliCtx.IsSet("provider") {
			providerAddr, err := address.NewFromString(cliCtx.String("provider"))
			if err != nil {
				return err
			}
			idAddr, err := fapi.StateLookupID(ctx, providerAddr, types.EmptyTSK)
			if err != nil {
				return err
			}
			provider, err = addressToActorID(idAddr)
			if err != nil {
				return err
			}
		}

		allocationWithIDs := make([]allocationWithID, 0, len(allocations))
		for id, allocation := range allocations {
			if provider != 0 && provider != allocation.Provider {
				continue
			}
			allocationWithIDs = append(allocationWithIDs, allocationWithID{
				id:         uint64(id),
				Allocation: allocation,
			})
		}
		sort.Slice(allocationWithIDs, func(i, j int) bool {
			return allocationWithIDs[i].id < allocationWithIDs[j].id
		})

		w := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
		fmt.Fprintln(w, "AllocationID\tClient\tProvider\tExpiration\tSize\tTermMin\tTermMax\tData")
		for _, allocation := range allocationWithIDs {
			fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%v\n", allocation.id, allocation.Client, allocation.Provider, allocation.Expiration,
				allocation.Size, allocation.TermMin, allocation.TermMax, allocation.Data)
		}

		return w.Flush()
	},
}

func addressToActorID(addr address.Address) (abi.ActorID, error) {
	if addr.Protocol() != address.ID {
		return 0, fmt.Errorf("%s not id address", addr)
	}
	id, err := strconv.ParseUint(addr.String()[2:], 10, 64)
	if err != nil {
		return 0, err
	}

	return abi.ActorID(id), nil
}
