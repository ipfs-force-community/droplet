package storageprovider

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ipfs-force-community/metrics"

	marketMetrics "github.com/filecoin-project/venus-market/v2/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-commp-utils/writer"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin/miner"

	minermgr2 "github.com/filecoin-project/venus-market/v2/minermgr"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	network2 "github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

// TODO: These are copied from spec-actors master, use spec-actors exports when we update
const DealMaxLabelSize = 256

type StorageDealHandler interface {
	AcceptDeal(ctx context.Context, deal *types.MinerDeal) error
	HandleOff(ctx context.Context, deal *types.MinerDeal) error
	HandleError(ctx context.Context, deal *types.MinerDeal, err error) error
	HandleReject(ctx context.Context, deal *types.MinerDeal, event storagemarket.StorageDealStatus, err error) error
}

var _ StorageDealHandler = (*StorageDealProcessImpl)(nil)

type StorageDealProcessImpl struct {
	metricsCtx metrics.MetricsCtx
	conns      *connmanager.ConnManager
	peerTagger network.PeerTagger
	spn        StorageProviderNode
	deals      repo.StorageDealRepo
	ask        IStorageAsk
	fs         filestore.FileStore
	stores     *stores.ReadWriteBlockstores
	dagStore   stores.DAGStoreWrapper // TODO:检查是否遗漏

	minerMgr        minermgr2.IAddrMgr
	pieceStorageMgr *piecestorage.PieceStorageManager
}

// NewStorageDealProcessImpl returns a new deal process instance
func NewStorageDealProcessImpl(
	metricsCtx metrics.MetricsCtx,
	conns *connmanager.ConnManager,
	peerTagger network.PeerTagger,
	spn StorageProviderNode,
	deals repo.StorageDealRepo,
	ask IStorageAsk,
	fs filestore.FileStore,
	minerMgr minermgr2.IAddrMgr,
	repo repo.Repo,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	dataTransfer network2.ProviderDataTransfer,
	dagStore stores.DAGStoreWrapper,
) (StorageDealHandler, error) {
	stores := stores.NewReadWriteBlockstores()

	err := dataTransfer.RegisterVoucherType(&requestvalidation.StorageDataTransferVoucher{}, requestvalidation.NewUnifiedRequestValidator(&providerPushDeals{deals}, nil))
	if err != nil {
		return nil, err
	}

	err = dataTransfer.RegisterTransportConfigurer(&requestvalidation.StorageDataTransferVoucher{}, dtutils.TransportConfigurer(newProviderStoreGetter(deals, stores)))
	if err != nil {
		return nil, err
	}

	return &StorageDealProcessImpl{
		metricsCtx: metricsCtx,
		conns:      conns,
		peerTagger: peerTagger,
		spn:        spn,
		deals:      deals,
		ask:        ask,
		fs:         fs,
		stores:     stores,

		minerMgr: minerMgr,

		pieceStorageMgr: pieceStorageMgr,
		dagStore:        dagStore,
	}, nil
}

// StorageDealUnknown->StorageDealValidating(ValidateDealProposal)->StorageDealAcceptWait(DecideOnProposal)->StorageDealWaitingForData
func (storageDealPorcess *StorageDealProcessImpl) AcceptDeal(ctx context.Context, minerDeal *types.MinerDeal) error {
	storageDealPorcess.peerTagger.TagPeer(minerDeal.Client, minerDeal.ProposalCid.String())

	tok, curEpoch, err := storageDealPorcess.spn.GetChainHead(ctx)
	if err != nil {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("node error getting most recent state id: %w", err))
	}

	if err := providerutils.VerifyProposal(ctx, minerDeal.ClientDealProposal, tok, storageDealPorcess.spn.VerifySignature); err != nil {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("verifying StorageDealProposal: %w", err))
	}

	proposal := minerDeal.Proposal

	// TODO: 判断 proposal.Provider 在本矿池中
	if !storageDealPorcess.minerMgr.Has(ctx, proposal.Provider) {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("incorrect provider for deal"))
	}

	if proposal.Label.Length() > DealMaxLabelSize {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("deal label can be at most %d bytes, is %d", DealMaxLabelSize, proposal.Label.Length()))
	}

	if err := proposal.PieceSize.Validate(); err != nil {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("proposal piece size is invalid: %w", err))
	}

	if !proposal.PieceCID.Defined() {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("proposal PieceCID undefined"))
	}

	if proposal.PieceCID.Prefix() != market.PieceCIDPrefix {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("proposal PieceCID had wrong prefix"))
	}

	if proposal.EndEpoch <= proposal.StartEpoch {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("proposal end before proposal start"))
	}

	if curEpoch > proposal.StartEpoch {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("deal start epoch has already elapsed"))
	}

	// Check that the delta between the start and end epochs (the deal
	// duration) is within acceptable bounds
	minDuration, maxDuration := market.DealDurationBounds(proposal.PieceSize)
	if proposal.Duration() < minDuration || proposal.Duration() > maxDuration {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("deal duration out of bounds (min, max, provided): %d, %d, %d", minDuration, maxDuration, proposal.Duration()))
	}

	// Check that the proposed end epoch isn't too far beyond the current epoch
	maxEndEpoch := curEpoch + miner.MaxSectorExpirationExtension
	if proposal.EndEpoch > maxEndEpoch {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("invalid deal end epoch %d: cannot be more than %d past current epoch %d", proposal.EndEpoch, miner.MaxSectorExpirationExtension, curEpoch))
	}

	pcMin, pcMax, err := storageDealPorcess.spn.DealProviderCollateralBounds(ctx, proposal.PieceSize, proposal.VerifiedDeal)
	if err != nil {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("node error getting collateral bounds: %w", err))
	}

	if proposal.ProviderCollateral.LessThan(pcMin) {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("proposed provider collateral below minimum: %s < %s", proposal.ProviderCollateral, pcMin))
	}

	if proposal.ProviderCollateral.GreaterThan(pcMax) {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("proposed provider collateral above maximum: %s > %s", proposal.ProviderCollateral, pcMax))
	}

	ask, err := storageDealPorcess.ask.GetAsk(ctx, proposal.Provider)
	if err != nil {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("failed to get ask for %s: %w", proposal.Provider, err))
	}

	askPrice := ask.Ask.Price
	if minerDeal.Proposal.VerifiedDeal {
		askPrice = ask.Ask.VerifiedPrice
	}

	minPrice := big.Div(big.Mul(askPrice, abi.NewTokenAmount(int64(proposal.PieceSize))), abi.NewTokenAmount(1<<30))
	if proposal.StoragePricePerEpoch.LessThan(minPrice) {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting,
			fmt.Errorf("storage price per epoch less than asking price: %s < %s", proposal.StoragePricePerEpoch, minPrice))
	}

	if proposal.PieceSize < ask.Ask.MinPieceSize {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting,
			fmt.Errorf("piece size less than minimum required size: %d < %d", proposal.PieceSize, ask.Ask.MinPieceSize))
	}

	if proposal.PieceSize > ask.Ask.MaxPieceSize {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting,
			fmt.Errorf("piece size more than maximum allowed size: %d > %d", proposal.PieceSize, ask.Ask.MaxPieceSize))
	}

	// check market funds
	clientMarketBalance, err := storageDealPorcess.spn.GetBalance(ctx, proposal.Client, tok)
	if err != nil {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("node error getting client market balance failed: %w", err))
	}

	// This doesn't guarantee that the client won't withdraw / lock those funds
	// but it's a decent first filter
	if clientMarketBalance.Available.LessThan(proposal.ClientBalanceRequirement()) {
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("clientMarketBalance.Available too small: %d < %d", clientMarketBalance.Available, proposal.ClientBalanceRequirement()))
	}

	// Verified deal checks
	if proposal.VerifiedDeal {
		dataCap, err := storageDealPorcess.spn.GetDataCap(ctx, proposal.Client, tok)
		if err != nil {
			return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("node error fetching verified data cap: %w", err))
		}
		if dataCap == nil {
			return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("node error fetching verified data cap: data cap missing -- client not verified"))
		}
		pieceSize := big.NewIntUnsigned(uint64(proposal.PieceSize))
		if dataCap.LessThan(pieceSize) {
			return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, fmt.Errorf("verified deal DataCap too small for proposed piece size"))
		}
	}

	err = storageDealPorcess.SendSignedResponse(ctx, proposal.Provider, &network.Response{
		State:    storagemarket.StorageDealWaitingForData,
		Proposal: minerDeal.ProposalCid,
	})
	if err != nil {
		return storageDealPorcess.HandleError(ctx, minerDeal, err)
	}

	if err := storageDealPorcess.conns.Disconnect(minerDeal.ProposalCid); err != nil {
		log.Warnf("closing client connection: %+v", err)
	}

	return storageDealPorcess.SaveState(ctx, minerDeal, storagemarket.StorageDealWaitingForData)
}

func (storageDealPorcess *StorageDealProcessImpl) HandleOff(ctx context.Context, deal *types.MinerDeal) error {
	// VerifyData
	if deal.State == storagemarket.StorageDealVerifyData {
		// finalize the blockstore as we're done writing deal data to it.
		if err := storageDealPorcess.FinalizeBlockstore(deal.ProposalCid); err != nil {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("failed to finalize read/write blockstore: %w", err))
		}

		pieceCid, metadataPath, err := storageDealPorcess.GeneratePieceCommitment(deal.ProposalCid, deal.InboundCAR, deal.Proposal.PieceSize)
		if err != nil {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("error generating CommP: %w", err))
		}

		// Verify CommP matches
		if pieceCid != deal.Proposal.PieceCID {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("proposal CommP doesn't match calculated CommP"))
		}

		deal.PiecePath = filestore.Path("")
		deal.MetadataPath = metadataPath
		deal.PieceStatus = types.Undefine

		deal.State = storagemarket.StorageDealReserveProviderFunds

		err = storageDealPorcess.deals.SaveDeal(ctx, deal)
		if err != nil {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
		}
	}

	// ReserveProviderFunds
	node := storageDealPorcess.spn
	if deal.State == storagemarket.StorageDealReserveProviderFunds {
		tok, _, err := storageDealPorcess.spn.GetChainHead(ctx)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("acquiring chain head: %w", err))
		}

		waddr, err := storageDealPorcess.spn.GetMinerWorkerAddress(ctx, deal.Proposal.Provider, tok)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("looking up miner worker: %w", err))
		}

		mcid, err := storageDealPorcess.spn.ReserveFunds(ctx, waddr, deal.Proposal.Provider, deal.Proposal.ProviderCollateral)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("reserving funds: %w", err))
		}

		if deal.FundsReserved.Nil() {
			deal.FundsReserved = deal.Proposal.ProviderCollateral
		} else {
			deal.FundsReserved = big.Add(deal.FundsReserved, deal.Proposal.ProviderCollateral)
		}

		// if no message was sent, and there was no error, funds were already available
		if mcid != cid.Undef {
			deal.AddFundsCid = &mcid
			deal.State = storagemarket.StorageDealProviderFunding

		} else {
			deal.State = storagemarket.StorageDealPublish // PublishDeal
		}

		err = storageDealPorcess.deals.SaveDeal(ctx, deal)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
		}
	}

	if deal.State == storagemarket.StorageDealProviderFunding { // WaitForFunding
		// TODO: 返回值处理
		errW := node.WaitForMessage(ctx, *deal.AddFundsCid, func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error {
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("AddFunds errored: %w", err))
			}
			if code != exitcode.Ok {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("AddFunds exit code: %s", code.String()))
			}
			deal.State = storagemarket.StorageDealPublish

			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
			}

			return nil
		})

		if errW != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("wait AddFunds msg for provider errored: %w", errW))
		}
	}

	if deal.State == storagemarket.StorageDealPublish {
		smDeal := types.MinerDeal{
			Client:             deal.Client,
			ClientDealProposal: deal.ClientDealProposal,
			ProposalCid:        deal.ProposalCid,
			State:              deal.State,
			Ref:                deal.Ref,
		}

		pdMCid, err := node.PublishDeals(ctx, smDeal)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("publishing deal: %w", err))
		}

		deal.PublishCid = &pdMCid

		deal.State = storagemarket.StorageDealPublishing
		err = storageDealPorcess.deals.SaveDeal(ctx, deal)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
		}
	}

	if deal.State == storagemarket.StorageDealPublishing { // WaitForPublish
		if deal.PublishCid != nil {
			res, err := storageDealPorcess.spn.WaitForPublishDeals(ctx, *deal.PublishCid, deal.Proposal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("PublishStorageDeals errored: %w", err))
			}

			// Once the deal has been published, release funds that were reserved
			// for deal publishing
			storageDealPorcess.releaseReservedFunds(ctx, deal)

			deal.DealID = res.DealID
			deal.PublishCid = &res.FinalCid
			deal.State = storagemarket.StorageDealStaged
			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
			}
		} else {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("state stop at StorageDealPublishing but not found publish cid"))
		}
	}

	if deal.State == storagemarket.StorageDealStaged { // HandoffDeal
		var carFilePath string
		if deal.PiecePath != "" {
			// Data for offline deals is stored on disk, so if PiecePath is set,
			// create a Reader from the file path
			file, err := storageDealPorcess.fs.Open(deal.PiecePath)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("reading piece at path %s: %w", deal.PiecePath, err))
			}
			carFilePath = string(file.OsPath())

			// Hand the deal off to the process that adds it to a sector
			log.Infow("handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			deal.PayloadSize = uint64(file.Size())
			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
			}
			err = storageDealPorcess.savePieceFile(ctx, deal, file, uint64(file.Size()))
			if err := file.Close(); err != nil {
				log.Errorw("failed to close imported CAR file", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid, "err", err)
			}

			if err != nil {
				err = fmt.Errorf("packing piece at path %s: %w", deal.PiecePath, err)
				return storageDealPorcess.HandleError(ctx, deal, err)
			}
		} else {
			carFilePath = deal.InboundCAR

			v2r, err := storageDealPorcess.ReadCAR(deal.InboundCAR)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("failed to open CARv2 file, proposalCid=%s: %w",
					deal.ProposalCid, err))
			}

			deal.PayloadSize = v2r.Header.DataSize
			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
			}
			dr, err := v2r.DataReader()
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("failed to get car data reader: %w", err))
			}
			// Hand the deal off to the process that adds it to a sector
			var packingErr error
			log.Infow("handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			packingErr = storageDealPorcess.savePieceFile(ctx, deal, dr, v2r.Header.DataSize)
			// Close the reader as we're done reading from it.
			if err := v2r.Close(); err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("failed to close CARv2 reader: %w", err))
			}
			log.Infow("closed car datareader after handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			if packingErr != nil {
				err = fmt.Errorf("packing piece %s: %w", deal.Ref.PieceCid, packingErr)
				return storageDealPorcess.HandleError(ctx, deal, err)
			}
		}

		// Register the deal data as a "shard" with the DAG store. Later it can be
		// fetched from the DAG store during retrieval.
		if err := stores.RegisterShardSync(ctx, storageDealPorcess.dagStore, deal.Proposal.PieceCID, carFilePath, true); err != nil {
			err = fmt.Errorf("failed to activate shard: %w", err)
			log.Error(err)
		} else {
			log.Infof("register shard successfully. deal:%d, proposalCid:%s, pieceCid:%s", deal.DealID, deal.ProposalCid, deal.Proposal.PieceCID)
		}

		log.Infow("successfully handed off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
		deal.AvailableForRetrieval = true
		deal.State = storagemarket.StorageDealAwaitingPreCommit
		if err := storageDealPorcess.deals.SaveDeal(ctx, deal); err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
		}
	}
	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) savePieceFile(ctx context.Context, deal *types.MinerDeal, reader io.Reader, payloadSize uint64) error {
	// because we use the PadReader directly during AP we need to produce the
	// correct amount of zeroes
	// (alternative would be to keep precise track of sector offsets for each
	// piece which is just too much work for a seldom used feature)

	pieceCid := deal.ClientDealProposal.Proposal.PieceCID

	_, err := storageDealPorcess.pieceStorageMgr.FindStorageForRead(ctx, pieceCid.String())
	if err != nil {
		ps, err := storageDealPorcess.pieceStorageMgr.FindStorageForWrite(int64(payloadSize))
		if err != nil {
			return err
		}
		_, err = ps.SaveTo(ctx, pieceCid.String(), reader)
		if err != nil {
			return err
		}
		_ = stats.RecordWithTags(storageDealPorcess.metricsCtx, []tag.Mutator{tag.Upsert(marketMetrics.StorageNameTag, ps.GetName())}, marketMetrics.StorageSaveHitCount.M(1))
		log.Infof("success to write file %s to piece storage", pieceCid)
	}
	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) SendSignedResponse(ctx context.Context, mAddr address.Address, resp *network.Response) error {
	s, err := storageDealPorcess.conns.DealStream(resp.Proposal)
	if err != nil {
		return fmt.Errorf("couldn't send response: %w", err)
	}

	respEx := &types.SignInfo{
		Data: resp,
		Type: vTypes.MTUnknown,
		Addr: mAddr,
	}
	sig, err := storageDealPorcess.spn.Sign(ctx, respEx)
	if err != nil {
		return fmt.Errorf("failed to sign response message: %w", err)
	}

	signedResponse := network.SignedResponse{
		Response:  *resp,
		Signature: sig,
	}

	// TODO: review ???
	err = s.WriteDealResponse(signedResponse, storageDealPorcess.spn.SignWithGivenMiner(mAddr))
	if err != nil {
		// Assume client disconnected
		_ = storageDealPorcess.conns.Disconnect(resp.Proposal)
	}
	return err
}

// StorageDealRejecting(RejectDeal)->StorageDealFailing(FailDeal)
func (storageDealPorcess *StorageDealProcessImpl) HandleReject(ctx context.Context, deal *types.MinerDeal, event storagemarket.StorageDealStatus, err error) error {
	deal.State = event
	deal.Message = err.Error()

	err = storageDealPorcess.SendSignedResponse(ctx, deal.Proposal.Provider, &network.Response{
		State:    storagemarket.StorageDealFailing,
		Message:  deal.Message,
		Proposal: deal.ProposalCid,
	})

	// ProviderEventSendResponseFailed/ProviderEventRejectionSent -> StorageDealFailing
	if err != nil {
		log.Errorf("failed response for reject: %s", err.Error())
	}

	// 断开连接
	if err = storageDealPorcess.conns.Disconnect(deal.ProposalCid); err != nil {
		log.Warnf("closing client connection: %+v", err)
	}

	storageDealPorcess.peerTagger.UntagPeer(deal.Client, deal.ProposalCid.String())

	return storageDealPorcess.deals.SaveDeal(ctx, deal)
}

func (storageDealPorcess *StorageDealProcessImpl) HandleError(ctx context.Context, deal *types.MinerDeal, err error) error {
	deal.State = storagemarket.StorageDealFailing
	deal.Message = err.Error()

	log.Errorf("deal %s failed: %s", deal.ProposalCid, deal.Message)

	storageDealPorcess.peerTagger.UntagPeer(deal.Client, deal.ProposalCid.String())

	if deal.PiecePath != filestore.Path("") {
		err := storageDealPorcess.fs.Delete(deal.PiecePath)
		if err != nil {
			log.Warnf("deleting piece at path %s: %w", deal.PiecePath, err)
		}
	}
	if deal.MetadataPath != filestore.Path("") {
		err := storageDealPorcess.fs.Delete(deal.MetadataPath)
		if err != nil {
			log.Warnf("deleting piece at path %s: %w", deal.MetadataPath, err)
		}
	}

	if deal.InboundCAR != "" {
		if err := storageDealPorcess.FinalizeBlockstore(deal.ProposalCid); err != nil {
			log.Warnf("error finalizing read-write store, car_path=%s: %s", deal.InboundCAR, err)
		}

		if err := storageDealPorcess.TerminateBlockstore(deal.ProposalCid, deal.InboundCAR); err != nil {
			log.Warnf("error deleting store, car_path=%s: %s", deal.InboundCAR, err)
		}
	}

	storageDealPorcess.releaseReservedFunds(ctx, deal)

	return storageDealPorcess.deals.SaveDeal(ctx, deal)
}

func (storageDealPorcess *StorageDealProcessImpl) releaseReservedFunds(ctx context.Context, deal *types.MinerDeal) {
	if !deal.FundsReserved.Nil() && !deal.FundsReserved.IsZero() {
		err := storageDealPorcess.spn.ReleaseFunds(ctx, deal.Proposal.Provider, deal.FundsReserved)
		if err != nil {
			// nonfatal error
			log.Warnf("failed to release funds: %s", err)
		}

		deal.FundsReserved = big.Zero() // TODO: big.Subtract(deal.FundsReserved, fundsReleased)
	}
}

func (storageDealPorcess *StorageDealProcessImpl) SaveState(ctx context.Context, deal *types.MinerDeal, event storagemarket.StorageDealStatus) error {
	deal.State = event
	return storageDealPorcess.deals.SaveDeal(ctx, deal)
}

func (storageDealPorcess *StorageDealProcessImpl) ReadCAR(path string) (*carv2.Reader, error) {
	return carv2.OpenReader(path)
}

func (storageDealPorcess *StorageDealProcessImpl) FinalizeBlockstore(proposalCid cid.Cid) error {
	bs, err := storageDealPorcess.stores.Get(proposalCid.String())
	if err != nil {
		return fmt.Errorf("failed to get read/write blockstore: %w", err)
	}

	if err := bs.Finalize(); err != nil {
		return fmt.Errorf("failed to finalize read/write blockstore: %w", err)
	}

	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) TerminateBlockstore(proposalCid cid.Cid, path string) error {
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
func (storageDealPorcess *StorageDealProcessImpl) GeneratePieceCommitment(proposalCid cid.Cid, carPath string, dealSize abi.PaddedPieceSize) (c cid.Cid, path filestore.Path, finalErr error) {
	rd, err := carv2.OpenReader(carPath)
	if err != nil {
		return cid.Undef, "", fmt.Errorf("failed to get CARv2 reader, proposalCid=%s, carPath=%s: %w", proposalCid, carPath, err)
	}

	defer func() {
		if err := rd.Close(); err != nil {
			log.Errorf("failed to close CARv2 reader, carPath=%s, err=%s", carPath, err)

			if finalErr == nil {
				c = cid.Undef
				path = ""
				finalErr = fmt.Errorf("failed to close CARv2 reader, proposalCid=%s, carPath=%s: %w",
					proposalCid, carPath, err)
				return
			}
		}
	}()

	// dump the CARv1 payload of the CARv2 file to the Commp Writer and get back the CommP.
	dr, err := rd.DataReader()
	if err != nil {
		return cid.Undef, "", fmt.Errorf("failed to get car data reader: %w", err)
	}
	w := &writer.Writer{}
	written, err := io.Copy(w, dr)
	if err != nil {
		return cid.Undef, "", fmt.Errorf("failed to write to CommP writer: %w", err)
	}
	if written != int64(rd.Header.DataSize) {
		return cid.Undef, "", fmt.Errorf("number of bytes written to CommP writer %d not equal to the CARv1 payload size %d", written, rd.Header.DataSize)
	}

	cidAndSize, err := w.Sum()
	if err != nil {
		return cid.Undef, "", fmt.Errorf("failed to get CommP: %w", err)
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
