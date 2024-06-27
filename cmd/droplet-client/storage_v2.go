package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/google/uuid"
	"github.com/ipfs-force-community/droplet/v2/api/clients/signer"
	cli2 "github.com/ipfs-force-community/droplet/v2/cli"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
)

var commonFlags = []cli.Flag{
	&cli.StringFlag{
		Name:     "provider",
		Usage:    "storage provider on-chain address",
		Required: true,
	},
	&cli.StringFlag{
		Name:  "from",
		Usage: "specify address to be used to initiate the deal",
	},
	&cli.IntFlag{
		Name:  "duration",
		Usage: "duration of the deal in epochs",
		Value: 518400, // default is 2880 * 180 == 180 days
	},
	&cli.IntFlag{
		Name:  "start-epoch-head-offset",
		Usage: "start epoch by when the deal should be proved by provider on-chain after current chain head",
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
	&cli.Int64Flag{
		Name:  "storage-price",
		Usage: "storage price in attoFIL per epoch per GiB",
		Value: 0,
	},
	&cli.Int64Flag{
		Name:  "provider-collateral",
		Usage: "specify the requested provider collateral the miner should put up",
		Value: 0,
	},
	&cli.BoolFlag{
		Name:  "remove-unsealed-copy",
		Usage: "indicates that an unsealed copy of the sector in not required for fast retrieval",
		Value: false,
	},
	&cli.BoolFlag{
		Name:  "skip-ipni-announce",
		Usage: "indicates that deal index should not be announced to the IPNI(Network Indexer)",
	},
	&cli2.CidBaseFlag,
}

var storageDealInitV2 = &cli.Command{
	Name:        "init-v2",
	Usage:       "Initialize storage offline deal with a miner, use v2 protocol",
	Description: "Make a deal with a miner.",
	ArgsUsage:   "[dataCid miner price duration]",
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:     "payload-cid",
			Usage:    "root CID of the CAR file",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "piece-cid",
			Usage:    "commp of the CAR file",
			Required: true,
		},
		&cli.Int64Flag{
			Name:     "piece-size",
			Usage:    "size of the CAR file as a padded piece",
			Required: true,
		},
	}, commonFlags...),
	Action: func(cctx *cli.Context) error {
		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

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

		params, err := commonParamsFromContext(cctx, fapi)
		if err != nil {
			return err
		}

		payloadCID, err := cid.Parse(cctx.String("payload-cid"))
		if err != nil {
			return err
		}

		addrInfo, err := cli2.GetAddressInfo(ctx, fapi, params.provider)
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
			return fmt.Errorf("cannot make a deal with storage provider %s because it does not support protocol version 1.2.0", params.provider)
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
		paddedPieceSize := abi.UnpaddedPieceSize(pieceSize).Padded()
		if params.isVerified && params.dcap.LessThan(abi.NewTokenAmount(int64(paddedPieceSize))) {
			return fmt.Errorf("not enough datacap to cover storage price: %v < %v", params.dcap, pieceSize)
		}

		var providerCollateral abi.TokenAmount
		if cctx.IsSet("provider-collateral") {
			providerCollateral = abi.NewTokenAmount(cctx.Int64("provider-collateral"))
		} else {
			bounds, err := fapi.StateDealProviderCollateralBounds(ctx, paddedPieceSize, params.isVerified, types.EmptyTSK)
			if err != nil {
				return fmt.Errorf("node error getting collateral bounds: %w", err)
			}

			providerCollateral = big.Div(big.Mul(bounds.Min, big.NewInt(6)), big.NewInt(5)) // add 20%
		}

		m := &manifest{
			payloadCID: payloadCID,
			pieceCID:   pieceCid,
			pieceSize:  abi.UnpaddedPieceSize(pieceSize),
		}

		if err = sendDeal(ctx, h, dealUuid, signer, params, addrInfo.ID, m, providerCollateral); err != nil {
			return err
		}

		msg := "sent deal proposal"
		msg += "\n"
		msg += fmt.Sprintf("  deal uuid: %s\n", dealUuid)
		msg += fmt.Sprintf("  storage provider: %s\n", params.provider)
		msg += fmt.Sprintf("  client: %s\n", params.from)
		msg += fmt.Sprintf("  payload cid: %s\n", payloadCID)
		msg += fmt.Sprintf("  piece cid: %s\n", pieceCid)
		msg += fmt.Sprintf("  start epoch: %d\n", params.startEpoch)
		msg += fmt.Sprintf("  end epoch: %d\n", params.startEpoch+params.duration)
		msg += fmt.Sprintf("  provider collateral: %s\n", types.FIL(providerCollateral).Short())
		fmt.Println(msg)

		return nil
	},
}

type commonParams struct {
	provider           address.Address
	from               address.Address
	duration           abi.ChainEpoch
	dcap               abi.TokenAmount
	isVerified         bool
	startEpoch         abi.ChainEpoch
	storagePrice       abi.TokenAmount
	removeUnsealedCopy bool
	skipIPNIAnnounce   bool
}

func commonParamsFromContext(cctx *cli.Context, fapi v1api.FullNode) (*commonParams, error) {
	ctx := cctx.Context
	params := &commonParams{}
	var err error
	params.provider, err = address.NewFromString(cctx.String("provider"))
	if err != nil {
		return nil, err
	}
	params.from, err = address.NewFromString(cctx.String("from"))
	if err != nil {
		return nil, err
	}
	params.duration = abi.ChainEpoch(cctx.Int64("duration"))

	// Check if the address is a verified client
	dcap, err := fapi.StateVerifiedClientStatus(ctx, params.from, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	var isVerified bool
	if dcap != nil {
		isVerified = true
		params.dcap = *dcap
	}

	// If the user has explicitly set the --verified-deal flag
	if cctx.IsSet("verified-deal") {
		// If --verified-deal is true, but the address is not a verified
		// client, return an error
		verifiedDealParam := cctx.Bool("verified-deal")
		if verifiedDealParam && !isVerified {
			return nil, fmt.Errorf("address %s does not have verified client status", params.from)
		}

		// Override the default
		isVerified = verifiedDealParam
	}
	params.isVerified = isVerified

	if cctx.IsSet("start-epoch") && cctx.IsSet("start-epoch-head-offset") {
		return nil, fmt.Errorf("only one flag from `start-epoch-head-offset' or `start-epoch` can be specified")
	}

	ts, err := fapi.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot get chain head: %w", err)
	}
	head := ts.Height()

	var startEpoch abi.ChainEpoch
	if cctx.IsSet("start-epoch-head-offset") {
		startEpoch = head + abi.ChainEpoch(cctx.Int("start-epoch-head-offset"))
	} else if cctx.IsSet("start-epoch") {
		startEpoch = abi.ChainEpoch(cctx.Int("start-epoch"))
	} else {
		// default
		startEpoch = head + abi.ChainEpoch(2880*8) // head + 8 days
	}
	params.startEpoch = startEpoch
	params.storagePrice = abi.NewTokenAmount(cctx.Int64("storage-price"))
	params.removeUnsealedCopy = cctx.Bool("remove-unsealed-copy")
	params.skipIPNIAnnounce = cctx.Bool("skip-ipni-announce")

	return params, nil
}

func sendDeal(ctx context.Context,
	h host.Host,
	dealUUID uuid.UUID,
	signer signer.ISigner,
	params *commonParams,
	peerID peer.ID,
	m *manifest,
	providerCollateral abi.TokenAmount,
) error {
	dealProposal, err := dealProposal(ctx, signer, params.from, m.payloadCID, m.pieceCID,
		m.pieceSize.Padded(), params.provider, params.startEpoch, params.duration, params.isVerified,
		providerCollateral, params.storagePrice)
	if err != nil {
		return err
	}
	transfer := types2.Transfer{
		Type: storagemarket.TTManual,
	}

	dealParams := types2.DealParams{
		DealUUID:           dealUUID,
		ClientDealProposal: *dealProposal,
		DealDataRoot:       m.payloadCID,
		IsOffline:          true,
		Transfer:           transfer,
		RemoveUnsealedCopy: params.removeUnsealedCopy,
		SkipIPNIAnnounce:   params.skipIPNIAnnounce,
	}

	log.Debugw("about to submit deal proposal", "uuid", dealUUID.String())

	s, err := h.NewStream(ctx, peerID, types2.DealProtocolv120ID)
	if err != nil {
		return fmt.Errorf("failed to open stream to peer %s: %w", peerID, err)
	}
	defer s.Close() // nolint

	var resp types2.DealResponse
	if err := doRpc(ctx, s, &dealParams, &resp); err != nil {
		return fmt.Errorf("send proposal rpc: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("deal proposal rejected: %s", resp.Message)
	}

	return nil
}

func dealProposal(ctx context.Context,
	signer signer.ISigner,
	clientAddr address.Address,
	rootCid cid.Cid,
	pieceCid cid.Cid,
	pieceSize abi.PaddedPieceSize,
	minerAddr address.Address,
	startEpoch abi.ChainEpoch,
	duration abi.ChainEpoch,
	verified bool,
	providerCollateral abi.TokenAmount,
	storagePrice abi.TokenAmount,
) (*types.ClientDealProposal, error) {
	endEpoch := startEpoch + duration
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

var batchStorageDealInitV2 = &cli.Command{
	Name:  "batch-init-v2",
	Usage: "batch init storage deal",
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:     "manifest",
			Usage:    "Path to the manifest file",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "output",
			Usage: "Path to the output file. If not specified, output will be `provider-date.csv`.",
		},
	}, commonFlags...),
	Action: func(cctx *cli.Context) error {
		ctx := cli2.ReqContext(cctx)
		fapi, fcloser, err := cli2.NewFullNode(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer fcloser()

		signer, scloser, err := cli2.GetSignerFromRepo(cctx, cli2.OldClientRepoPath)
		if err != nil {
			return err
		}
		defer scloser()

		params, err := commonParamsFromContext(cctx, fapi)
		if err != nil {
			return err
		}

		addrInfo, err := cli2.GetAddressInfo(ctx, fapi, params.provider)
		if err != nil {
			return err
		}

		h, err := cli2.NewHost(cctx, cli2.OldClientRepoPath)
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
			return fmt.Errorf("cannot make a deal with storage provider %s because it does not support protocol version 1.2.0", params.provider)
		}

		manifests, err := loadManifest(cctx.String("manifest"))
		if err != nil {
			return fmt.Errorf("load manifest error: %v", err)
		}

		output := cctx.String("output")
		if output == "" {
			output = fmt.Sprintf("%s-%s.csv", params.provider, time.Now().Format("2006-01-02-15-04-05"))
		}

		buf := &bytes.Buffer{}
		writer := csv.NewWriter(buf)
		_ = writer.Write([]string{"DealUUID", "Provider", "Client", "PieceCID", "PieceSize", "PayloadCID"})

		defer func() {
			writer.Flush()
			_ = os.WriteFile(output, buf.Bytes(), 0o644)
		}()

		dcap := params.dcap.Int
		for _, m := range manifests {
			paddedPieceSize := m.pieceSize.Padded()
			dcap = big.NewInt(0).Sub(dcap, big.NewInt(int64(paddedPieceSize)).Int)
			if dcap.Cmp(big.NewInt(0).Int) < 0 {
				fmt.Printf("not enough datacap to create deal: %v\n", dcap)
				break
			}

			dealUUID := uuid.New()
			var providerCollateral abi.TokenAmount
			if cctx.IsSet("provider-collateral") {
				providerCollateral = abi.NewTokenAmount(cctx.Int64("provider-collateral"))
			} else {
				bounds, err := fapi.StateDealProviderCollateralBounds(ctx, paddedPieceSize, params.isVerified, types.EmptyTSK)
				if err != nil {
					return fmt.Errorf("node error getting collateral bounds: %w", err)
				}

				providerCollateral = big.Div(big.Mul(bounds.Min, big.NewInt(6)), big.NewInt(5)) // add 20%
			}

			if err := sendDeal(ctx, h, dealUUID, signer, params, addrInfo.ID, m, providerCollateral); err != nil {
				return err
			}
			fmt.Println("created deal", dealUUID, ", piece cid", m.pieceCID)

			_ = writer.Write([]string{dealUUID.String(), params.provider.String(), params.from.String(),
				m.pieceCID.String(), fmt.Sprintf("%d", paddedPieceSize), m.payloadCID.String()})
		}

		return nil
	},
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

		resp, err := sendDealStatusRequest(ctx, s, dealUUID, from, signer)
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
