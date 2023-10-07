package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/api/clients/signer"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
	"github.com/ipfs/go-cid"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
)

var storageDealInitV2 = &cli.Command{
	Name:  "init-v2",
	Usage: "Initialize storage offline deal with a miner, use v2 protocol",
	Description: `Make a deal with a miner.
dataCid comes from running 'droplet-client data import'.
miner is the address of the miner you wish to make a deal with.
price is measured in FIL/Epoch. Miners usually don't accept a bid
lower than their advertised ask (which is in FIL/GiB/Epoch). You can check a miners listed price
with './droplet-client storage asks query <miner address>'.
duration is how long the miner should store the data for, in blocks.
The minimum value is 518400 (6 months).`,
	ArgsUsage: "[dataCid miner price duration]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "piece-cid",
			Usage: "manually specify piece commitment for data (dataCid must be to a car file)",
		},
		&cli.Int64Flag{
			Name:  "piece-size",
			Usage: "if manually specifying piece cid, used to specify size (dataCid must be to a car file)",
		},
		&cli.BoolFlag{
			Name:  "manual-stateless-deal",
			Usage: "instructs the node to send an offline deal without registering it with the deallist/fsm",
		},
		&cli.StringFlag{
			Name:  "from",
			Usage: "specify address to fund the deal with",
		},
		&cli.Int64Flag{
			Name:  "start-epoch",
			Usage: "specify the epoch that the deal should start at",
		},
		&cli.BoolFlag{
			Name:  "fast-retrieval",
			Usage: "indicates that data should be available for fast retrieval",
			Value: true,
		},
		&cli.BoolFlag{
			Name:        "verified-deal",
			Usage:       "indicate that the deal counts towards verified client total",
			DefaultText: "true if client is verified, false otherwise",
		},
		&cli.StringFlag{
			Name:  "provider-collateral",
			Usage: "specify the requested provider collateral the miner should put up",
		},
		&cli2.CidBaseFlag,
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

		h, err := cli2.NewHost(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}

		signer, scloser, err := cli2.GetSignerFromRepo(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer scloser()

		ctx := cli2.ReqContext(cctx)

		if cctx.NArg() != 4 {
			return errors.New("expected 4 args: dataCid, miner, price, duration")
		}

		// [data, miner, price, dur]
		p, err := dealParamsFromContext(cctx, api, fapi, false)
		if err != nil {
			return err
		}

		data, err := cid.Parse(p.firstArg)
		if err != nil {
			return err
		}

		addrInfo, err := cli2.GetAddressInfo(ctx, fapi, p.miner[0])
		if err != nil {
			return err
		}

		if err := h.Connect(ctx, *addrInfo); err != nil {
			return fmt.Errorf("failed to connect to peer %s: %w", addrInfo.ID, err)
		}
		x, err := h.Peerstore().FirstSupportedProtocol(addrInfo.ID, types2.DealProtocolv121ID)
		if err != nil {
			return fmt.Errorf("getting protocols for peer %s: %w", addrInfo.ID, err)
		}

		if len(x) == 0 {
			return fmt.Errorf("cannot make a deal with storage provider %s because it does not support protocol version 1.2.0", p.miner[0])
		}

		dealUuid := uuid.New()

		pieceCidStr := cctx.String("piece-cid")
		pieceCid, err := cid.Parse(pieceCidStr)
		if err != nil {
			return fmt.Errorf("parsing commp '%s': %w", pieceCidStr, err)
		}

		pieceSize := cctx.Uint64("piece-size")
		if pieceSize == 0 {
			return fmt.Errorf("must provide piece-size parameter for CAR url")
		}

		transfer := types2.Transfer{
			Type: storagemarket.TTManual,
		}

		var providerCollateral abi.TokenAmount
		if cctx.IsSet("provider-collateral") {
			providerCollateral = abi.NewTokenAmount(cctx.Int64("provider-collateral"))
		} else {
			bounds, err := fapi.StateDealProviderCollateralBounds(ctx, abi.PaddedPieceSize(pieceSize), p.isVerified, types.EmptyTSK)
			if err != nil {
				return fmt.Errorf("node error getting collateral bounds: %w", err)
			}

			providerCollateral = big.Div(big.Mul(bounds.Min, big.NewInt(6)), big.NewInt(5)) // add 20%
		}

		tipset, err := fapi.ChainHead(ctx)
		if err != nil {
			return fmt.Errorf("cannot get chain head: %w", err)
		}
		head := tipset.Height()

		var startEpoch abi.ChainEpoch
		if cctx.IsSet("start-epoch") {
			startEpoch = abi.ChainEpoch(cctx.Int("start-epoch"))
		} else {
			startEpoch = head + abi.ChainEpoch(2880*8) // head + 8 days
		}

		dealProposal, err := dealProposal(ctx, signer, p.from, data, pieceCid, abi.UnpaddedPieceSize(pieceSize).Padded(),
			p.miner[0], startEpoch, int(p.dur), p.isVerified, providerCollateral, abi.NewTokenAmount(0))
		if err != nil {
			return err
		}

		dealParams := types2.DealParams{
			DealUUID:           dealUuid,
			ClientDealProposal: *dealProposal,
			DealDataRoot:       data,
			IsOffline:          true,
			Transfer:           transfer,
			RemoveUnsealedCopy: false,
			SkipIPNIAnnounce:   false,
		}

		log.Debugw("about to submit deal proposal", "uuid", dealUuid.String())

		s, err := h.NewStream(ctx, addrInfo.ID, types2.DealProtocolv120ID)
		if err != nil {
			return fmt.Errorf("failed to open stream to peer %s: %w", addrInfo.ID, err)
		}
		defer s.Close() // nolint

		var resp types2.DealResponse
		if err := doRpc(ctx, s, &dealParams, &resp); err != nil {
			return fmt.Errorf("send proposal rpc: %w", err)
		}

		if !resp.Accepted {
			return fmt.Errorf("deal proposal rejected: %s", resp.Message)
		}

		fmt.Println("deal uuid: ", dealUuid)

		proposalNd, err := cborutil.AsIpld(dealProposal)
		if err == nil {
			fmt.Println("proposal cid: ", proposalNd.Cid())
		}

		return nil
	},
}

func dealProposal(ctx context.Context,
	signer signer.ISigner,
	clientAddr address.Address,
	rootCid cid.Cid,
	pieceCid cid.Cid,
	pieceSize abi.PaddedPieceSize,
	minerAddr address.Address,
	startEpoch abi.ChainEpoch,
	duration int,
	verified bool,
	providerCollateral abi.TokenAmount,
	storagePrice abi.TokenAmount,
) (*types.ClientDealProposal, error) {
	endEpoch := startEpoch + abi.ChainEpoch(duration)
	// deal proposal expects total storage price for deal per epoch, therefore we
	// multiply pieceSize * storagePrice (which is set per epoch per GiB) and divide by 2^30
	storagePricePerEpochForDeal := big.Div(big.Mul(big.NewInt(int64(pieceSize)), storagePrice), big.NewInt(int64(1<<30)))
	l, err := types.NewLabelFromString(rootCid.String())
	if err != nil {
		return nil, err
	}
	proposal := types.DealProposal{
		PieceCID:             pieceCid,
		PieceSize:            pieceSize,
		VerifiedDeal:         verified,
		Client:               clientAddr,
		Provider:             minerAddr,
		Label:                l,
		StartEpoch:           startEpoch,
		EndEpoch:             endEpoch,
		StoragePricePerEpoch: storagePricePerEpochForDeal,
		ProviderCollateral:   providerCollateral,
	}

	buf, err := cborutil.Dump(&proposal)
	if err != nil {
		return nil, err
	}

	sig, err := signer.WalletSign(ctx, clientAddr, buf, types.MsgMeta{Type: types.MTDealProposal, Extra: buf})
	if err != nil {
		return nil, fmt.Errorf("wallet sign failed: %w", err)
	}

	return &types.ClientDealProposal{
		Proposal:        proposal,
		ClientSignature: *sig,
	}, nil
}

func doRpc(ctx context.Context, s inet.Stream, req interface{}, resp interface{}) error {
	errc := make(chan error)
	go func() {
		if err := cborutil.WriteCborRPC(s, req); err != nil {
			errc <- fmt.Errorf("failed to send request: %w", err)
			return
		}

		if err := cborutil.ReadCborRPC(s, resp); err != nil {
			errc <- fmt.Errorf("failed to read response: %w", err)
			return
		}

		errc <- nil
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

var storageDealStatus = &cli.Command{
	Name:  "status",
	Usage: "search deal status by libp2p",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "provider",
			Usage:    "storage provider on-chain address",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "deal-uuid",
			Usage:    "",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "from",
			Usage: "the address that was used to sign the deal proposal",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context

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

		h, err := cli2.NewHost(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}

		signer, scloser, err := cli2.GetSignerFromRepo(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer scloser()

		dealUUID, err := uuid.Parse(cctx.String("deal-uuid"))
		if err != nil {
			return err
		}
		maddr, err := address.NewFromString(cctx.String("provider"))
		if err != nil {
			return err
		}
		from, err := cli2.AddressFromContextOrDefault(cctx, api)
		if err != nil {
			return err
		}

		addrInfo, err := cli2.GetAddressInfo(ctx, fapi, maddr)
		if err != nil {
			return err
		}

		if err := h.Connect(ctx, *addrInfo); err != nil {
			return fmt.Errorf("failed to connect to peer %s: %w", addrInfo.ID, err)
		}
		s, err := h.NewStream(ctx, addrInfo.ID, types2.DealStatusV12ProtocolID)
		if err != nil {
			return err
		}
		defer s.Close() // nolint

		resp, err := sendDealStatusRequest(ctx, s, addrInfo.ID, dealUUID, from, signer)
		if err != nil {
			return fmt.Errorf("send deal status request failed: %w", err)
		}

		var lstr string
		if resp != nil && resp.DealStatus != nil {
			label := resp.DealStatus.Proposal.Label
			if label.IsString() {
				lstr, err = label.ToString()
				if err != nil {
					lstr = "could not marshall deal label"
				}
				dataCid, err := cid.Decode(lstr)
				if err == nil {
					lstr = dataCid.String()
				}
			}
		}

		msg := "got deal status response"
		msg += "\n"

		if resp.Error != "" {
			msg += fmt.Sprintf("  error: %s\n", resp.Error)
			fmt.Println(msg)
			return nil
		}

		msg += fmt.Sprintf("  deal    uuid: %s\n", resp.DealUUID)
		msg += fmt.Sprintf("  piece    cid: %s\n", resp.DealStatus.Proposal.PieceCID)
		msg += fmt.Sprintf("  piece   size: %d\n", resp.DealStatus.Proposal.PieceSize)
		msg += fmt.Sprintf("  proposal cid: %s\n", resp.DealStatus.SignedProposalCid)
		msg += fmt.Sprintf("  deal   state: %s\n", resp.DealStatus.Status)
		msg += fmt.Sprintf("  deal  status: %s\n", resp.DealStatus.SealingStatus)
		msg += fmt.Sprintf("  deal   label: %s\n", lstr)
		msg += fmt.Sprintf("  publish  cid: %s\n", resp.DealStatus.PublishCid)
		msg += fmt.Sprintf("  deal      id: %d\n", resp.DealStatus.ChainDealID)
		fmt.Println(msg)

		return nil
	},
}

func sendDealStatusRequest(ctx context.Context,
	s inet.Stream,
	id peer.ID,
	dealUUID uuid.UUID,
	from address.Address,
	signer signer.ISigner,
) (*types2.DealStatusResponse, error) {
	uuidBytes, err := dealUUID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("getting uuid bytes: %w", err)
	}

	// todo: create a new MsgType
	sig, err := signer.WalletSign(ctx, from, uuidBytes, types.MsgMeta{Type: types.MTUnknown, Extra: uuidBytes})
	if err != nil {
		return nil, fmt.Errorf("signing uuid bytes: %w", err)
	}

	// Set a deadline on writing to the stream so it doesn't hang
	_ = s.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer s.SetWriteDeadline(time.Time{}) // nolint

	// Write the deal status request to the stream
	req := types2.DealStatusRequest{DealUUID: dealUUID, Signature: *sig}
	if err = cborutil.WriteCborRPC(s, &req); err != nil {
		return nil, fmt.Errorf("sending deal status req: %w", err)
	}

	// Set a deadline on reading from the stream so it doesn't hang
	_ = s.SetReadDeadline(time.Now().Add(60 * time.Second))
	defer s.SetReadDeadline(time.Time{}) // nolint

	// Read the response from the stream
	var resp types2.DealStatusResponse
	if err := resp.UnmarshalCBOR(s); err != nil {
		return nil, fmt.Errorf("reading deal status response: %w", err)
	}

	return &resp, nil
}
