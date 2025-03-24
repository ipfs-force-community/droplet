package storageprovider

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	"github.com/ipfs-force-community/metrics"

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
	"github.com/filecoin-project/go-state-types/builtin/v10/market"
	"github.com/filecoin-project/go-state-types/builtin/v12/miner"
	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/ipfs-force-community/droplet/v2/config"
	marketMetrics "github.com/ipfs-force-community/droplet/v2/metrics"
	"github.com/ipfs-force-community/droplet/v2/minermgr"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	network2 "github.com/ipfs-force-community/droplet/v2/network"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"

	vTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/venus/venus-shared/actors/policy"
)

// TODO: These are copied from spec-actors master, use spec-actors exports when we update
const DealMaxLabelSize = 256

type StorageDealHandler interface {
	AcceptLegacyDeal(ctx context.Context, deal *types.MinerDeal) error
	AcceptNewDeal(ctx context.Context, minerDeal *types.MinerDeal) error
	HandleOff(ctx context.Context, deal *types.MinerDeal) error
	HandleError(ctx context.Context, deal *types.MinerDeal, err error) error
	HandleReject(ctx context.Context, deal *types.MinerDeal, event storagemarket.StorageDealStatus, err error) error
}

var _ StorageDealHandler = (*StorageDealProcessImpl)(nil)

type StorageDealProcessImpl struct {
	metricsCtx     metrics.MetricsCtx
	conns          *connmanager.ConnManager
	peerTagger     network.PeerTagger
	spn            StorageProviderNode
	deals          repo.StorageDealRepo
	ask            IStorageAsk
	tf             config.TransferFileStoreConfigFunc
	stores         *stores.ReadWriteBlockstores
	dagStore       stores.DAGStoreWrapper
	eventPublisher *EventPublishAdapter

	minerMgr        minermgr.IMinerMgr
	pieceStorageMgr *piecestorage.PieceStorageManager

	sdf config.StorageDealFilter
}

// NewStorageDealProcessImpl returns a new deal process instance
func NewStorageDealProcessImpl(
	metricsCtx metrics.MetricsCtx,
	conns *connmanager.ConnManager,
	peerTagger network.PeerTagger,
	spn StorageProviderNode,
	deals repo.StorageDealRepo,
	ask IStorageAsk,
	tf config.TransferFileStoreConfigFunc,
	minerMgr minermgr.IMinerMgr,
	pieceStorageMgr *piecestorage.PieceStorageManager,
	dataTransfer network2.ProviderDataTransfer,
	dagStore stores.DAGStoreWrapper,
	sdf config.StorageDealFilter,
	pb *EventPublishAdapter,
) (StorageDealHandler, error) {
	err := dataTransfer.RegisterVoucherType(requestvalidation.StorageDataTransferVoucherType, requestvalidation.NewUnifiedRequestValidator(&providerPushDeals{deals}, nil))
	if err != nil {
		return nil, err
	}

	blockstores := stores.NewReadWriteBlockstores()
	err = dataTransfer.RegisterTransportConfigurer(requestvalidation.StorageDataTransferVoucherType, dtutils.TransportConfigurer(newProviderStoreGetter(deals, blockstores)))
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
		tf:         tf,
		stores:     blockstores,

		minerMgr: minerMgr,

		pieceStorageMgr: pieceStorageMgr,
		dagStore:        dagStore,
		eventPublisher:  pb,
		sdf:             sdf,
	}, nil
}

func (storageDealPorcess *StorageDealProcessImpl) runDealDecisionLogic(ctx context.Context, minerDeal *types.MinerDeal) (bool, string, error) {
	if storageDealPorcess.sdf == nil {
		return true, "", nil
	}
	return storageDealPorcess.sdf(ctx, minerDeal.Proposal.Provider, minerDeal)
}

// StorageDealUnknown->StorageDealValidating(ValidateDealProposal)->StorageDealAcceptWait(DecideOnProposal)->StorageDealWaitingForData
func (storageDealPorcess *StorageDealProcessImpl) AcceptLegacyDeal(ctx context.Context, minerDeal *types.MinerDeal) error {
	storageDealPorcess.peerTagger.TagPeer(minerDeal.Client, minerDeal.ProposalCid.String())
	err := storageDealPorcess.acceptDeal(ctx, minerDeal)
	if err != nil {
		if strings.Contains(err.Error(), nodeErrStr) {
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, minerDeal)
		}
		return storageDealPorcess.HandleReject(ctx, minerDeal, storagemarket.StorageDealRejecting, err)
	}

	err = storageDealPorcess.SendSignedResponse(ctx, minerDeal.Proposal.Provider, &network.Response{
		State:    storagemarket.StorageDealWaitingForData,
		Proposal: minerDeal.ProposalCid,
	})
	if err != nil {
		return storageDealPorcess.HandleError(ctx, minerDeal, err)
	}

	storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealAccepted, minerDeal)

	if err := storageDealPorcess.conns.Disconnect(minerDeal.ProposalCid); err != nil {
		log.Warnf("closing client connection: %+v", err)
	}

	return storageDealPorcess.SaveState(ctx, minerDeal, storagemarket.StorageDealWaitingForData)
}

func (storageDealPorcess *StorageDealProcessImpl) AcceptNewDeal(ctx context.Context, minerDeal *types.MinerDeal) error {
	return storageDealPorcess.acceptDeal(ctx, minerDeal)
}

var nodeErrStr = "node error:"

func (storageDealPorcess *StorageDealProcessImpl) acceptDeal(ctx context.Context, minerDeal *types.MinerDeal) error {
	tok, curEpoch, err := storageDealPorcess.spn.GetChainHead(ctx)
	if err != nil {
		return fmt.Errorf("%s getting most recent state id: %w", nodeErrStr, err)
	}

	if err := providerutils.VerifyProposal(ctx, minerDeal.ClientDealProposal, tok, storageDealPorcess.spn.VerifySignature); err != nil {
		return fmt.Errorf("verifying StorageDealProposal: %w", err)
	}

	proposal := minerDeal.Proposal

	if !storageDealPorcess.minerMgr.Has(ctx, proposal.Provider) {
		return fmt.Errorf("incorrect provider for deal")
	}

	if proposal.Label.Length() > DealMaxLabelSize {
		return fmt.Errorf("deal label can be at most %d bytes, is %d", DealMaxLabelSize, proposal.Label.Length())
	}

	if err := proposal.PieceSize.Validate(); err != nil {
		return fmt.Errorf("proposal piece size is invalid: %w", err)
	}

	if !proposal.PieceCID.Defined() {
		return fmt.Errorf("proposal PieceCID undefined")
	}

	if proposal.PieceCID.Prefix() != market.PieceCIDPrefix {
		return fmt.Errorf("proposal PieceCID had wrong prefix")
	}

	if proposal.EndEpoch <= proposal.StartEpoch {
		return fmt.Errorf("proposal end before proposal start")
	}

	if curEpoch > proposal.StartEpoch {
		return fmt.Errorf("deal start epoch has already elapsed")
	}

	// Check that the delta between the start and end epochs (the deal
	// duration) is within acceptable bounds
	minDuration, maxDuration := policy.DealDurationBounds(proposal.PieceSize)
	if proposal.Duration() < minDuration || proposal.Duration() > maxDuration {
		return fmt.Errorf("deal duration out of bounds (min, max, provided): %d, %d, %d", minDuration, maxDuration, proposal.Duration())
	}

	// Check that the proposed end epoch isn't too far beyond the current epoch
	maxEndEpoch := curEpoch + miner.MaxSectorExpirationExtension
	if proposal.EndEpoch > maxEndEpoch {
		return fmt.Errorf("invalid deal end epoch %d: cannot be more than %d past current epoch %d", proposal.EndEpoch, miner.MaxSectorExpirationExtension, curEpoch)
	}

	pcMin, pcMax, err := storageDealPorcess.spn.DealProviderCollateralBounds(ctx, proposal.Provider, proposal.PieceSize, proposal.VerifiedDeal)
	if err != nil {
		return fmt.Errorf("%s getting collateral bounds: %w", nodeErrStr, err)
	}

	if proposal.ProviderCollateral.LessThan(pcMin) {
		return fmt.Errorf("proposed provider collateral below minimum: %s < %s", proposal.ProviderCollateral, pcMin)
	}

	if proposal.ProviderCollateral.GreaterThan(pcMax) {
		return fmt.Errorf("proposed provider collateral above maximum: %s > %s", proposal.ProviderCollateral, pcMax)
	}

	ask, err := storageDealPorcess.ask.GetAsk(ctx, proposal.Provider)
	if err != nil {
		return fmt.Errorf("failed to get ask for %s: %w", proposal.Provider, err)
	}

	askPrice := ask.Ask.Price
	if minerDeal.Proposal.VerifiedDeal {
		askPrice = ask.Ask.VerifiedPrice
	}

	minPrice := big.Div(big.Mul(askPrice, abi.NewTokenAmount(int64(proposal.PieceSize))), abi.NewTokenAmount(1<<30))
	if proposal.StoragePricePerEpoch.LessThan(minPrice) {
		return fmt.Errorf("storage price per epoch less than asking price: %s < %s", proposal.StoragePricePerEpoch, minPrice)
	}

	if proposal.PieceSize < ask.Ask.MinPieceSize {
		return fmt.Errorf("piece size less than minimum required size: %d < %d", proposal.PieceSize, ask.Ask.MinPieceSize)
	}

	if proposal.PieceSize > ask.Ask.MaxPieceSize {
		return fmt.Errorf("piece size more than maximum allowed size: %d > %d", proposal.PieceSize, ask.Ask.MaxPieceSize)
	}

	// check market funds
	clientMarketBalance, err := storageDealPorcess.spn.GetBalance(ctx, proposal.Client, tok)
	if err != nil {
		return fmt.Errorf("%s getting client market balance failed: %w", nodeErrStr, err)
	}

	// This doesn't guarantee that the client won't withdraw / lock those funds
	// but it's a decent first filter
	if clientMarketBalance.Available.LessThan(proposal.ClientBalanceRequirement()) {
		return fmt.Errorf("clientMarketBalance.Available too small: %d < %d", clientMarketBalance.Available, proposal.ClientBalanceRequirement())
	}

	// Verified deal checks
	if proposal.VerifiedDeal {
		dataCap, err := storageDealPorcess.spn.GetDataCap(ctx, proposal.Client, tok)
		if err != nil {
			return fmt.Errorf("%s fetching verified data cap: %w", nodeErrStr, err)
		}
		if dataCap == nil {
			return fmt.Errorf("%s fetching verified data cap: data cap missing -- client not verified", nodeErrStr)
		}
		pieceSize := big.NewIntUnsigned(uint64(proposal.PieceSize))
		if dataCap.LessThan(pieceSize) {
			return fmt.Errorf("verified deal DataCap too small for proposed piece size")
		}
	}

	storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealDeciding, minerDeal)
	accept, reason, err := storageDealPorcess.runDealDecisionLogic(ctx, minerDeal)
	if err != nil {
		return fmt.Errorf("custom deal decision logic failed: %w", err)
	}

	if !accept {
		return fmt.Errorf(reason)
	}

	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) HandleOff(ctx context.Context, deal *types.MinerDeal) error {
	// VerifyData
	if deal.State == storagemarket.StorageDealVerifyData {

		handleErr := func(err error) error {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDataVerificationFailed, deal)
			return storageDealPorcess.HandleError(ctx, deal, err)
		}

		// finalize the blockstore as we're done writing deal data to it.
		if err := storageDealPorcess.FinalizeBlockstore(deal.ProposalCid); err != nil {
			err = fmt.Errorf("failed to finalize read/write blockstore: %w", err)
			return handleErr(err)
		}

		pieceCid, metadataPath, err := storageDealPorcess.GeneratePieceCommitment(deal.ProposalCid, deal.InboundCAR, deal.Proposal.PieceSize)
		if err != nil {
			err = fmt.Errorf("error generating CommP: %w", err)
			return handleErr(err)
		}

		// Verify CommP matches
		if pieceCid != deal.Proposal.PieceCID {
			err = fmt.Errorf("proposal CommP doesn't match calculated CommP")
			return handleErr(err)
		}

		deal.PiecePath = filestore.Path("")
		deal.MetadataPath = metadataPath
		deal.PieceStatus = types.Undefine

		deal.State = storagemarket.StorageDealReserveProviderFunds

		err = storageDealPorcess.deals.SaveDeal(ctx, deal)
		if err != nil {
			err = fmt.Errorf("fail to save deal to database: %w", err)
			return handleErr(err)
		}

		storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventVerifiedData, deal)
	}

	// ReserveProviderFunds
	node := storageDealPorcess.spn
	if deal.State == storagemarket.StorageDealReserveProviderFunds {
		tok, _, err := storageDealPorcess.spn.GetChainHead(ctx)
		if err != nil {
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("acquiring chain head: %w", err))
		}

		waddr, err := storageDealPorcess.spn.GetMinerWorkerAddress(ctx, deal.Proposal.Provider, tok)
		if err != nil {
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("looking up miner worker: %w", err))
		}

		mcid, err := storageDealPorcess.spn.ReserveFunds(ctx, waddr, deal.Proposal.Provider, deal.Proposal.ProviderCollateral)
		if err != nil {
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("reserving funds: %w", err))
		}
		storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventFundsReserved, deal)

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
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventFunded, deal)
			deal.State = storagemarket.StorageDealPublish // PublishDeal
		}

		storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventFundingInitiated, deal)
		err = storageDealPorcess.deals.SaveDeal(ctx, deal)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
		}
	}

	if deal.State == storagemarket.StorageDealProviderFunding { // WaitForFunding
		// TODO: 返回值处理
		errW := node.WaitForMessage(ctx, *deal.AddFundsCid, func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error {
			if err != nil {
				storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
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

		storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventFunded, deal)
		if errW != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("wait AddFunds msg for provider errored: %w", errW))
		}
	}

	if deal.State == storagemarket.StorageDealPublish {
		log.Debugf("publish deal %s", deal.ProposalCid)
		smDeal := types.MinerDeal{
			Client:             deal.Client,
			ClientDealProposal: deal.ClientDealProposal,
			ProposalCid:        deal.ProposalCid,
			State:              deal.State,
			Ref:                deal.Ref,
		}

		pdMCid, err := node.PublishDeals(ctx, smDeal)
		if err != nil {
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealPublishError, deal)
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("publishing deal: %w", err))
		}
		storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealPublishInitiated, deal)
		deal.PublishCid = &pdMCid

		deal.State = storagemarket.StorageDealPublishing
		err = storageDealPorcess.deals.SaveDeal(ctx, deal)
		if err != nil {
			return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database"))
		}
	}

	if deal.State == storagemarket.StorageDealPublishing { // WaitForPublish
		log.Debugf("wait for publish deal %s, publishCid: %s", deal.ProposalCid, deal.PublishCid)
		if deal.PublishCid != nil {
			res, err := storageDealPorcess.spn.WaitForPublishDeals(ctx, *deal.PublishCid, deal.Proposal)
			if err != nil {
				storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
				storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealPublishError, deal)
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("PublishStorageDeals errored: %w", err))
			}
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealPublished, deal)

			// Once the deal has been published, release funds that were reserved
			// for deal publishing
			storageDealPorcess.releaseReservedFunds(ctx, deal)
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventFundsReleased, deal)

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
			fs, err := storageDealPorcess.tf(deal.Proposal.Provider)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("get temp file store for %s: %w", deal.Proposal.Provider, err))
			}
			file, err := fs.Open(deal.PiecePath)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("reading piece at path %s: %w", deal.PiecePath, err))
			}
			carFilePath = string(file.OsPath())

			// Hand the deal off to the process that adds it to a sector
			log.Infow("handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			deal.PayloadSize = uint64(file.Size())
			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database: %v", err))
			}
			err = storageDealPorcess.savePieceFile(ctx, deal, file, uint64(file.Size()))
			if err := file.Close(); err != nil {
				log.Errorw("failed to close imported CAR file", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid, "err", err)
			}

			if err != nil {
				err = fmt.Errorf("packing piece at path %s: %w", deal.PiecePath, err)
				return storageDealPorcess.HandleError(ctx, deal, err)
			}
		} else if len(deal.InboundCAR) != 0 {
			carFilePath = deal.InboundCAR

			v2r, err := storageDealPorcess.ReadCAR(deal.InboundCAR)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("failed to open CARv2 file, proposalCid=%s: %w",
					deal.ProposalCid, err))
			}

			deal.PayloadSize = v2r.Header.DataSize
			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database: %v", err))
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
		} else {
			// An index can be created even if carFilePath is empty
			carFilePath = ""
			// carfile may already in piece storage, verify it
			pieceStore, err := storageDealPorcess.pieceStorageMgr.FindStorageForRead(ctx, deal.Proposal.PieceCID.String())
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, err)
			}
			log.Debugf("found %s in piece storage", deal.Proposal.PieceCID)

			l, err := pieceStore.Len(ctx, deal.Proposal.PieceCID.String())
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to got payload size: %v", err))
			}

			deal.PayloadSize = uint64(l)
			err = storageDealPorcess.deals.SaveDeal(ctx, deal)
			if err != nil {
				return storageDealPorcess.HandleError(ctx, deal, fmt.Errorf("fail to save deal to database: %v", err))
			}
		}
		log.Infof("after publishing deal. piece cid: %s, payload size: %d", deal.Proposal.PieceCID, deal.PayloadSize)

		go func() {
			log.Infof("register shard. deal: %d, proposalCid: %s, pieceCid: %s", deal.DealID, deal.ProposalCid, deal.Proposal.PieceCID)
			// Register the deal data as a "shard" with the DAG store. Later it can be
			// fetched from the DAG store during retrieval.
			if err := storageDealPorcess.dagStore.RegisterShard(ctx, deal.Proposal.PieceCID, carFilePath, true, nil); err != nil {
				log.Errorf("failed to register shard: %v", err)
			}
		}()

		// Remove temporary car files
		storageDealPorcess.removeTemporaryFile(ctx, deal, true)

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

func (storageDealPorcess *StorageDealProcessImpl) removeTemporaryFile(ctx context.Context, deal *types.MinerDeal, checkPieceStorage bool) {
	if checkPieceStorage {
		// Check if the temporary file has been copied to piece storage
		_, err := storageDealPorcess.pieceStorageMgr.FindStorageForRead(ctx, deal.Proposal.PieceCID.String())
		if err != nil {
			log.Warnf("failed to delete temporary file: %v, %v", deal.ProposalCid, err)
			return
		}
	}

	err := func() error {
		if deal.PiecePath != filestore.Path("") {
			fs, err := storageDealPorcess.tf(deal.Proposal.Provider)
			if err != nil {
				return fmt.Errorf("get temp file store for %s: %v", deal.Proposal.Provider, err)
			}
			err = fs.Delete(deal.PiecePath)
			if err != nil {
				return fmt.Errorf("deleting piece at path %s: %v", deal.PiecePath, err)
			}
		}
		if deal.MetadataPath != filestore.Path("") {
			fs, err := storageDealPorcess.tf(deal.Proposal.Provider)
			if err != nil {
				return fmt.Errorf("get temp file store for %s: %v", deal.Proposal.Provider, err)
			}
			err = fs.Delete(deal.MetadataPath)
			if err != nil {
				return fmt.Errorf("deleting piece at path %s: %v", deal.MetadataPath, err)
			}
		}
		if deal.InboundCAR != "" {
			if err := storageDealPorcess.TerminateBlockstore(deal.ProposalCid, deal.InboundCAR); err != nil {
				return fmt.Errorf("error deleting store, car_path=%s: %s", deal.InboundCAR, err)
			}
		}
		return nil
	}()
	if err != nil {
		log.Warnf("failed to delete temporary file: %v, %v", deal.ProposalCid, err)
	} else {
		log.Infof("delete temporary file success: %v", deal.ProposalCid)
	}
}

func (storageDealPorcess *StorageDealProcessImpl) SendSignedResponse(ctx context.Context, mAddr address.Address, resp *network.Response) error {
	s, err := storageDealPorcess.conns.DealStream(resp.Proposal)
	if err != nil {
		return fmt.Errorf("couldn't send response: %w", err)
	}

	respEx := &types.SignInfo{
		Data: resp,
		Type: vTypes.MTNetWorkResponse,
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

	err = s.WriteDealResponse(signedResponse, nil)
	if err != nil {
		// Assume client disconnected
		_ = storageDealPorcess.conns.Disconnect(resp.Proposal)
	}
	return err
}

// StorageDealRejecting(RejectDeal)->StorageDealFailing(FailDeal)
func (storageDealPorcess *StorageDealProcessImpl) HandleReject(ctx context.Context, deal *types.MinerDeal, event storagemarket.StorageDealStatus, err error) error {
	storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventDealRejected, deal)
	log.Infof("deal rejected (proposal cid: %s ): %s", deal.ProposalCid, err)

	deal.State = event
	deal.Message = err.Error()

	err = storageDealPorcess.SendSignedResponse(ctx, deal.Proposal.Provider, &network.Response{
		State:    storagemarket.StorageDealFailing,
		Message:  deal.Message,
		Proposal: deal.ProposalCid,
	})

	storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventRejectionSent, deal)
	// ProviderEventSendResponseFailed/ProviderEventRejectionSent -> StorageDealFailing
	if err != nil {
		log.Errorf("failed response for reject: %s", err.Error())
	}

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

	storageDealPorcess.removeTemporaryFile(ctx, deal, false)

	storageDealPorcess.releaseReservedFunds(ctx, deal)

	return storageDealPorcess.deals.SaveDeal(ctx, deal)
}

func (storageDealPorcess *StorageDealProcessImpl) releaseReservedFunds(ctx context.Context, deal *types.MinerDeal) {
	if !deal.FundsReserved.Nil() && !deal.FundsReserved.IsZero() {
		err := storageDealPorcess.spn.ReleaseFunds(ctx, deal.Proposal.Provider, deal.FundsReserved)
		if err != nil {
			// nonfatal error
			storageDealPorcess.eventPublisher.Publish(storagemarket.ProviderEventNodeErrored, deal)
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
