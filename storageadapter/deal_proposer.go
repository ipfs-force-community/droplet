package storageadapter

import (
	"context"
	"github.com/filecoin-project/go-commp-utils/writer"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	market2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v5/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"
	"io"
	"os"
)

// TODO: These are copied from spec-actors master, use spec-actors exports when we update
const DealMaxLabelSize = 256

type PeerTager struct {
	net network.StorageMarketNetwork
}

func (p *PeerTager) TagPeer(id peer.ID, s string) {
	p.net.TagPeer(id, s)
}

func (p *PeerTager) UntagPeer(id peer.ID, s string) {
	p.net.UntagPeer(id, s)
}

type StorageDealProcess interface {
	AcceptDeal(ctx context.Context, deal *storagemarket.MinerDeal) error
	HandleOff(ctx context.Context, deal *storagemarket.MinerDeal) error
	HandleError(deal *storagemarket.MinerDeal, err error) error
	HandleReject(deal *storagemarket.MinerDeal, event storagemarket.StorageDealStatus, err error) error
}

var _ StorageDealProcess = (*StorageDealPorcess)(nil)

type StorageDealPorcess struct {
	conns     *connmanager.ConnManager
	peerTager network.PeerTagger
	spn       StorageProviderNode
	deals     StorageDealStore
	ask       StorageAsk
	stores    *stores.ReadWriteBlockstores
}

func (storageDealPorcess *StorageDealPorcess) AcceptDeal(ctx context.Context, minerDeal *storagemarket.MinerDeal) error {
	storageDealPorcess.peerTager.TagPeer(minerDeal.Client, minerDeal.ProposalCid.String())

	tok, curEpoch, err := storageDealPorcess.spn.GetChainHead(ctx)
	if err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("node error getting most recent state id: %w", err))
	}

	if err := providerutils.VerifyProposal(ctx, minerDeal.ClientDealProposal, tok, storageDealPorcess.spn.VerifySignature); err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("verifying StorageDealProposal: %w", err))
	}

	proposal := minerDeal.Proposal

	//todo validate is this miner deals in support list
	/*if proposal.Provider != environment.Address() {
		return storageDealPorcess.(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("incorrect provider for deal"))
	}*/

	if len(proposal.Label) > DealMaxLabelSize {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("deal label can be at most %d bytes, is %d", DealMaxLabelSize, len(proposal.Label)))
	}

	if err := proposal.PieceSize.Validate(); err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("proposal piece size is invalid: %w", err))
	}

	if !proposal.PieceCID.Defined() {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("proposal PieceCID undefined"))
	}

	if proposal.PieceCID.Prefix() != market.PieceCIDPrefix {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("proposal PieceCID had wrong prefix"))
	}

	if proposal.EndEpoch <= proposal.StartEpoch {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("proposal end before proposal start"))
	}

	if curEpoch > proposal.StartEpoch {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("deal start epoch has already elapsed"))
	}

	// Check that the delta between the start and end epochs (the deal
	// duration) is within acceptable bounds
	minDuration, maxDuration := market2.DealDurationBounds(proposal.PieceSize)
	if proposal.Duration() < minDuration || proposal.Duration() > maxDuration {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("deal duration out of bounds (min, max, provided): %d, %d, %d", minDuration, maxDuration, proposal.Duration()))
	}

	// Check that the proposed end epoch isn't too far beyond the current epoch
	maxEndEpoch := curEpoch + miner.MaxSectorExpirationExtension
	if proposal.EndEpoch > maxEndEpoch {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("invalid deal end epoch %d: cannot be more than %d past current epoch %d", proposal.EndEpoch, miner.MaxSectorExpirationExtension, curEpoch))
	}

	pcMin, pcMax, err := storageDealPorcess.spn.DealProviderCollateralBounds(ctx, proposal.PieceSize, proposal.VerifiedDeal)
	if err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("node error getting collateral bounds: %w", err))
	}

	if proposal.ProviderCollateral.LessThan(pcMin) {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("proposed provider collateral below minimum: %s < %s", proposal.ProviderCollateral, pcMin))
	}

	if proposal.ProviderCollateral.GreaterThan(pcMax) {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("proposed provider collateral above maximum: %s > %s", proposal.ProviderCollateral, pcMax))
	}
	ask, err := storageDealPorcess.ask.GetAsk(proposal.Provider)
	if err != nil {

	}
	askPrice := ask.Ask.Price
	if minerDeal.Proposal.VerifiedDeal {
		askPrice = ask.Ask.VerifiedPrice
	}

	minPrice := big.Div(big.Mul(askPrice, abi.NewTokenAmount(int64(proposal.PieceSize))), abi.NewTokenAmount(1<<30))
	if proposal.StoragePricePerEpoch.LessThan(minPrice) {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting,
			xerrors.Errorf("storage price per epoch less than asking price: %s < %s", proposal.StoragePricePerEpoch, minPrice))
	}

	if proposal.PieceSize < ask.Ask.MinPieceSize {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting,
			xerrors.Errorf("piece size less than minimum required size: %d < %d", proposal.PieceSize, ask.Ask.MinPieceSize))
	}

	if proposal.PieceSize > ask.Ask.MaxPieceSize {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting,
			xerrors.Errorf("piece size more than maximum allowed size: %d > %d", proposal.PieceSize, ask.Ask.MaxPieceSize))
	}

	// check market funds
	clientMarketBalance, err := storageDealPorcess.spn.GetBalance(ctx, proposal.Client, tok)
	if err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("node error getting client market balance failed: %w", err))
	}

	// This doesn't guarantee that the client won't withdraw / lock those funds
	// but it's a decent first filter
	if clientMarketBalance.Available.LessThan(proposal.ClientBalanceRequirement()) {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("clientMarketBalance.Available too small: %d < %d", clientMarketBalance.Available, proposal.ClientBalanceRequirement()))
	}

	// Verified deal checks
	if proposal.VerifiedDeal {
		dataCap, err := storageDealPorcess.spn.GetDataCap(ctx, proposal.Client, tok)
		if err != nil {
			return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("node error fetching verified data cap: %w", err))
		}
		if dataCap == nil {
			return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("node error fetching verified data cap: data cap missing -- client not verified"))
		}
		pieceSize := big.NewIntUnsigned(uint64(proposal.PieceSize))
		if dataCap.LessThan(pieceSize) {
			return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("verified deal DataCap too small for proposed piece size"))
		}
	}

	// Send intent to accept
	err = storageDealPorcess.SendSignedResponse(ctx, &network.Response{
		State:    storagemarket.StorageDealWaitingForData,
		Proposal: minerDeal.ProposalCid,
	})
	if err != nil {
		return storageDealPorcess.HandleError(minerDeal, err)
	}

	if err := storageDealPorcess.conns.Disconnect(minerDeal.ProposalCid); err != nil {
		log.Warnf("closing client connection: %+v", err)
	}

	return storageDealPorcess.SaveState(minerDeal, storagemarket.StorageDealWaitingForData)
}

func (storageDealPorcess *StorageDealPorcess) HandleOff(ctx context.Context, deal *storagemarket.MinerDeal) error {
	//VerifyData
	// finalize the blockstore as we're done writing deal data to it.
	if deal.State == storagemarket.StorageDealReserveProviderFunds {
		if err := storageDealPorcess.FinalizeBlockstore(deal.ProposalCid); err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("failed to finalize read/write blockstore: %w", err))
		}

		pieceCid, metadataPath, err := storageDealPorcess.GeneratePieceCommitment(deal.ProposalCid, deal.InboundCAR, deal.Proposal.PieceSize)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("error generating CommP: %w", err))
		}

		// Verify CommP matches
		if pieceCid != deal.Proposal.PieceCID {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("proposal CommP doesn't match calculated CommP"))
		}
		deal.State = storagemarket.StorageDealReserveProviderFunds
		deal.MetadataPath = metadataPath
		err = storageDealPorcess.deals.SaveDeal(deal)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("fail to save deal to database"))
		}
	}

	//ReserveProviderFunds
	if deal.AddFundsCid != nil && *deal.AddFundsCid == cid.Undef {
		tok, _, err := storageDealPorcess.spn.GetChainHead(ctx)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("acquiring chain head: %w", err))
		}

		waddr, err := storageDealPorcess.spn.GetMinerWorkerAddress(ctx, deal.Proposal.Provider, tok)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("looking up miner worker: %w", err))
		}

		mcid, err := storageDealPorcess.spn.ReserveFunds(ctx, waddr, deal.Proposal.Provider, deal.Proposal.ProviderCollateral)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("reserving funds: %w", err))
		}

		if deal.FundsReserved.Nil() {
			deal.FundsReserved = deal.Proposal.ProviderCollateral
		} else {
			deal.FundsReserved = big.Add(deal.FundsReserved, deal.Proposal.ProviderCollateral)
		}

		if mcid != cid.Undef {
			deal.State = storagemarket.StorageDealProviderFunding
		}

		err = storageDealPorcess.deals.SaveDeal(deal)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("fail to save deal to database"))
		}
	}

	//WaitForFunding
	if deal.AddFundsCid != nil && deal.State == storagemarket.StorageDealProviderFunding {
		err := storageDealPorcess.spn.WaitForMessage(ctx, *deal.AddFundsCid, func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error {
			if err != nil {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("AddFunds errored: %w", err))
			}
			if code != exitcode.Ok {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("AddFunds exit code: %s", code.String()))
			}
			deal.State = storagemarket.StorageDealPublish
			err = storageDealPorcess.deals.SaveDeal(deal)
			if err != nil {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("fail to save deal to database"))
			}
			return nil
		})
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("fail to save deal to database"))
		}
	}
	//PublishDeal

	//WaitForPublish

	//HandoffDeal
	panic("implement me")
}

func (storageDealPorcess *StorageDealPorcess) SendSignedResponse(ctx context.Context, resp *network.Response) error {
	s, err := storageDealPorcess.conns.DealStream(resp.Proposal)
	if err != nil {
		return xerrors.Errorf("couldn't send response: %w", err)
	}

	sig, err := storageDealPorcess.spn.Sign(ctx, resp)
	if err != nil {
		return xerrors.Errorf("failed to sign response message: %w", err)
	}

	signedResponse := network.SignedResponse{
		Response:  *resp,
		Signature: sig,
	}

	err = s.WriteDealResponse(signedResponse, storageDealPorcess.spn.Sign)
	if err != nil {
		// Assume client disconnected
		_ = storageDealPorcess.conns.Disconnect(resp.Proposal)
	}
	return err
}

func (storageDealPorcess *StorageDealPorcess) HandleReject(deal *storagemarket.MinerDeal, event storagemarket.StorageDealStatus, err error) error {
	deal.State = event
	deal.Message = err.Error()
	return storageDealPorcess.deals.SaveDeal(deal)
}

func (storageDealPorcess *StorageDealPorcess) HandleError(deal *storagemarket.MinerDeal, err error) error {
	deal.State = storagemarket.StorageDealFailing
	deal.Message = err.Error()
	return storageDealPorcess.deals.SaveDeal(deal)
}

func (storageDealPorcess *StorageDealPorcess) SaveState(deal *storagemarket.MinerDeal, event storagemarket.StorageDealStatus) error {
	deal.State = event
	return storageDealPorcess.deals.SaveDeal(deal)
}

func (storageDealPorcess *StorageDealPorcess) ReadCAR(path string) (*carv2.Reader, error) {
	return carv2.OpenReader(path)
}

func (storageDealPorcess *StorageDealPorcess) FinalizeBlockstore(proposalCid cid.Cid) error {
	bs, err := storageDealPorcess.stores.Get(proposalCid.String())
	if err != nil {
		return xerrors.Errorf("failed to get read/write blockstore: %w", err)
	}

	if err := bs.Finalize(); err != nil {
		return xerrors.Errorf("failed to finalize read/write blockstore: %w", err)
	}

	return nil
}

func (storageDealPorcess *StorageDealPorcess) TerminateBlockstore(proposalCid cid.Cid, path string) error {
	// stop tracking it.
	if err := storageDealPorcess.stores.Untrack(proposalCid.String()); err != nil {
		log.Warnf("failed to untrack read write blockstore, proposalCid=%s, car_path=%s: %s", proposalCid, path, err)
	}

	// delete the backing CARv2 file as it was a temporary file we created for
	// this storage deal; the piece has now been handed off, or the deal has failed.
	if err := os.Remove(path); err != nil {
		log.Warnf("failed to delete carv2 file on termination, car_path=%s: %s", path, err)
	}

	return nil
}

// GeneratePieceCommitment generates the pieceCid for the CARv1 deal payload in
// the CARv2 file that already exists at the given path.
func (storageDealPorcess *StorageDealPorcess) GeneratePieceCommitment(proposalCid cid.Cid, carPath string, dealSize abi.PaddedPieceSize) (c cid.Cid, path filestore.Path, finalErr error) {
	rd, err := carv2.OpenReader(carPath)
	if err != nil {
		return cid.Undef, "", xerrors.Errorf("failed to get CARv2 reader, proposalCid=%s, carPath=%s: %w", proposalCid, carPath, err)
	}

	defer func() {
		if err := rd.Close(); err != nil {
			log.Errorf("failed to close CARv2 reader, carPath=%s, err=%s", carPath, err)

			if finalErr == nil {
				c = cid.Undef
				path = ""
				finalErr = xerrors.Errorf("failed to close CARv2 reader, proposalCid=%s, carPath=%s: %w",
					proposalCid, carPath, err)
				return
			}
		}
	}()

	// dump the CARv1 payload of the CARv2 file to the Commp Writer and get back the CommP.
	w := &writer.Writer{}
	written, err := io.Copy(w, rd.DataReader())
	if err != nil {
		return cid.Undef, "", xerrors.Errorf("failed to write to CommP writer: %w", err)
	}
	if written != int64(rd.Header.DataSize) {
		return cid.Undef, "", xerrors.Errorf("number of bytes written to CommP writer %d not equal to the CARv1 payload size %d", written, rd.Header.DataSize)
	}

	cidAndSize, err := w.Sum()
	if err != nil {
		return cid.Undef, "", xerrors.Errorf("failed to get CommP: %w", err)
	}

	if cidAndSize.PieceSize < dealSize {
		// need to pad up!
		rawPaddedCommp, err := commp.PadCommP(
			// we know how long a pieceCid "hash" is, just blindly extract the trailing 32 bytes
			cidAndSize.PieceCID.Hash()[len(cidAndSize.PieceCID.Hash())-32:],
			uint64(cidAndSize.PieceSize),
			uint64(dealSize),
		)
		if err != nil {
			return cid.Undef, "", err
		}
		cidAndSize.PieceCID, _ = commcid.DataCommitmentV1ToCID(rawPaddedCommp)
	}

	return cidAndSize.PieceCID, filestore.Path(""), err
}
