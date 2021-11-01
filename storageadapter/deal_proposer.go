package storageadapter

import (
	"context"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-state-types/exitcode"
	minermgr2 "github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-market/piece"
	"io"
	"os"

	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"golang.org/x/xerrors"

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
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	market2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v5/actors/builtin/miner"

	network2 "github.com/filecoin-project/venus-market/network"
)

// TODO: These are copied from spec-actors master, use spec-actors exports when we update
const DealMaxLabelSize = 256

type StorageDealProcess interface {
	AcceptDeal(ctx context.Context, deal *storagemarket.MinerDeal) error
	HandleOff(ctx context.Context, deal *storagemarket.MinerDeal) error
	HandleError(deal *storagemarket.MinerDeal, err error) error
	HandleReject(deal *storagemarket.MinerDeal, event storagemarket.StorageDealStatus, err error) error
}

var _ StorageDealProcess = (*StorageDealProcessImpl)(nil)

type StorageDealProcessImpl struct {
	conns      *connmanager.ConnManager
	peerTagger network.PeerTagger
	spn        StorageProviderNode
	deals      StorageDealStore
	ask        IStorageAsk
	fs         filestore.FileStore
	stores     *stores.ReadWriteBlockstores

	pieceStore piecestore.PieceStore // TODO:检查是否遗漏

	dagStore stores.DAGStoreWrapper // TODO:检查是否遗漏

	minerMgr minermgr2.IMinerMgr
	storage  piece.IPieceStorage
}

// NewStorageDealProcessImpl returns a new deal process instance
func NewStorageDealProcessImpl(
	conns *connmanager.ConnManager,
	peerTagger network.PeerTagger,
	spn StorageProviderNode,
	deals StorageDealStore,
	ask IStorageAsk,
	fs filestore.FileStore,
	minerMgr minermgr2.IMinerMgr,
	pieceStore piecestore.PieceStore,
	dataTransfer network2.ProviderDataTransfer,
	dagStore stores.DAGStoreWrapper,
) (StorageDealProcess, error) {
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
		conns:      conns,
		peerTagger: peerTagger,
		spn:        spn,
		deals:      deals,
		ask:        ask,
		fs:         fs,
		stores:     stores,

		minerMgr: minerMgr,

		pieceStore: pieceStore,
		dagStore:   dagStore,
	}, nil
}

// StorageDealUnknown->StorageDealValidating(ValidateDealProposal)->StorageDealAcceptWait(DecideOnProposal)->StorageDealWaitingForData
func (storageDealPorcess *StorageDealProcessImpl) AcceptDeal(ctx context.Context, minerDeal *storagemarket.MinerDeal) error {
	storageDealPorcess.peerTagger.TagPeer(minerDeal.Client, minerDeal.ProposalCid.String())

	tok, curEpoch, err := storageDealPorcess.spn.GetChainHead(ctx)
	if err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("node error getting most recent state id: %w", err))
	}

	if err := providerutils.VerifyProposal(ctx, minerDeal.ClientDealProposal, tok, storageDealPorcess.spn.VerifySignature); err != nil {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("verifying StorageDealProposal: %w", err))
	}

	proposal := minerDeal.Proposal

	// TODO: 判断 proposal.Provider 在本矿池中
	if !storageDealPorcess.minerMgr.Has(ctx, proposal.Provider) {
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("incorrect provider for deal"))
	}

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
		return storageDealPorcess.HandleReject(minerDeal, storagemarket.StorageDealRejecting, xerrors.Errorf("failed to get ask for %s: %w", proposal.Provider, err))
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

	// TODO: RunCustomDecisionLogic ?

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

func (storageDealPorcess *StorageDealProcessImpl) HandleOff(ctx context.Context, deal *storagemarket.MinerDeal) error {
	// VerifyData
	if deal.State == storagemarket.StorageDealVerifyData {
		// finalize the blockstore as we're done writing deal data to it.
		if err := storageDealPorcess.FinalizeBlockstore(deal.ProposalCid); err != nil {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("failed to finalize read/write blockstore: %w", err))
		}

		pieceCid, metadataPath, err := storageDealPorcess.GeneratePieceCommitment(deal.ProposalCid, deal.InboundCAR, deal.Proposal.PieceSize)
		if err != nil {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("error generating CommP: %w", err))
		}

		// Verify CommP matches
		if pieceCid != deal.Proposal.PieceCID {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("proposal CommP doesn't match calculated CommP"))
		}

		deal.PiecePath = filestore.Path("")
		deal.MetadataPath = metadataPath
		deal.State = storagemarket.StorageDealReserveProviderFunds

		err = storageDealPorcess.deals.SaveDeal(deal)
		if err != nil {
			deal.PiecePath = filestore.Path("")
			deal.MetadataPath = filestore.Path("")
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("fail to save deal to database"))
		}
	}

	// ReserveProviderFunds
	node := storageDealPorcess.spn
	if deal.State == storagemarket.StorageDealReserveProviderFunds {
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

		// if no message was sent, and there was no error, funds were already available
		if mcid != cid.Undef {
			deal.AddFundsCid = &mcid

			deal.State = storagemarket.StorageDealProviderFunding // WaitForFunding
			// TODO: 返回值处理
			errW := node.WaitForMessage(ctx, *deal.AddFundsCid, func(code exitcode.ExitCode, bytes []byte, finalCid cid.Cid, err error) error {
				if err != nil {
					return storageDealPorcess.HandleError(deal, xerrors.Errorf("AddFunds errored: %w", err))
				}
				if code != exitcode.Ok {
					return storageDealPorcess.HandleError(deal, xerrors.Errorf("AddFunds exit code: %s", code.String()))
				}
				deal.State = storagemarket.StorageDealPublish

				return nil
			})

			if errW != nil {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("Wait AddFunds msg for Provider errored: %w", err))
			}
		}

		deal.State = storagemarket.StorageDealPublish // PublishDeal
		smDeal := storagemarket.MinerDeal{
			Client:             deal.Client,
			ClientDealProposal: deal.ClientDealProposal,
			ProposalCid:        deal.ProposalCid,
			State:              deal.State,
			Ref:                deal.Ref,
		}

		pdMCid, err := node.PublishDeals(ctx, smDeal)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("publishing deal: %w", err))
		}

		deal.PublishCid = &pdMCid

		deal.State = storagemarket.StorageDealPublishing // WaitForPublish
		res, err := storageDealPorcess.spn.WaitForPublishDeals(ctx, *deal.PublishCid, deal.Proposal)
		if err != nil {
			return storageDealPorcess.HandleError(deal, xerrors.Errorf("PublishStorageDeals errored: %w", err))
		}

		// Once the deal has been published, release funds that were reserved
		// for deal publishing
		storageDealPorcess.releaseReservedFunds(ctx, deal)

		deal.DealID = res.DealID
		deal.PublishCid = &res.FinalCid

		deal.State = storagemarket.StorageDealStaged // HandoffDeal
		var carFilePath string
		if deal.PiecePath != "" {
			// Data for offline deals is stored on disk, so if PiecePath is set,
			// create a Reader from the file path
			file, err := storageDealPorcess.fs.Open(deal.PiecePath)
			if err != nil {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("reading piece at path %s: %w", deal.PiecePath, err))
			}
			carFilePath = string(file.OsPath())

			// Hand the deal off to the process that adds it to a sector
			log.Infow("handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			err = storageDealPorcess.savePieceFile(ctx, deal, file, uint64(file.Size()))
			if err := file.Close(); err != nil {
				log.Errorw("failed to close imported CAR file", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid, "err", err)
			}

			if err != nil {
				err = xerrors.Errorf("packing piece at path %s: %w", deal.PiecePath, err)
				return storageDealPorcess.HandleError(deal, err)
			}
		} else {
			carFilePath = deal.InboundCAR

			v2r, err := storageDealPorcess.ReadCAR(deal.InboundCAR)
			if err != nil {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("failed to open CARv2 file, proposalCid=%s: %w",
					deal.ProposalCid, err))
			}

			// Hand the deal off to the process that adds it to a sector
			var packingErr error
			log.Infow("handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			packingErr = storageDealPorcess.savePieceFile(ctx, deal, v2r.DataReader(), v2r.Header.DataSize)
			// Close the reader as we're done reading from it.
			if err := v2r.Close(); err != nil {
				return storageDealPorcess.HandleError(deal, xerrors.Errorf("failed to close CARv2 reader: %w", err))
			}
			log.Infow("closed car datareader after handing off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
			if packingErr != nil {
				err = xerrors.Errorf("packing piece %s: %w", deal.Ref.PieceCid, packingErr)
				return storageDealPorcess.HandleError(deal, err)
			}
		}

		if err := storageDealPorcess.savePieceMetadata(deal); err != nil {
			err = xerrors.Errorf("failed to register deal data for piece %s for retrieval: %w", deal.Ref.PieceCid, err)
			log.Error(err.Error())
			return storageDealPorcess.HandleError(deal, err)
		}

		// Register the deal data as a "shard" with the DAG store. Later it can be
		// fetched from the DAG store during retrieval.
		if err := stores.RegisterShardSync(ctx, storageDealPorcess.dagStore, deal.Proposal.PieceCID, carFilePath, true); err != nil {
			err = xerrors.Errorf("failed to activate shard: %w", err)
			log.Error(err)
		}

		log.Infow("successfully handed off deal to sealing subsystem", "pieceCid", deal.Proposal.PieceCID, "proposalCid", deal.ProposalCid)
		deal.AvailableForRetrieval = true
		deal.State = storagemarket.StorageDealAwaitingPreCommit
	}

	//todo should be async to do
	//add timer to check P2 C2 status
	// VerifyDealPreCommitted
	if deal.State == storagemarket.StorageDealAwaitingPreCommit {
		cb := func(sectorNumber abi.SectorNumber, isActive bool, err error) {
			// It's possible that
			// - we miss the pre-commit message and have to wait for prove-commit
			// - the deal is already active (for example if the node is restarted
			//   while waiting for pre-commit)
			// In either of these two cases, isActive will be true.
			switch {
			case err != nil:
				storageDealPorcess.HandleError(deal, err)
			case isActive:
				deal.State = storagemarket.StorageDealFinalizing
			default:
				{
					deal.SectorNumber = sectorNumber

					deal.State = storagemarket.StorageDealSealing // VerifyDealActivated
					// TODO: consider waiting for seal to happen
					cbDSC := func(err error) {
						if err != nil {
							storageDealPorcess.HandleError(deal, err)
						} else {
							deal.State = storagemarket.StorageDealFinalizing // CleanupDeal
							if deal.PiecePath != "" {
								err := storageDealPorcess.fs.Delete(deal.PiecePath)
								if err != nil {
									log.Warnf("deleting piece at path %s: %w", deal.PiecePath, err)
								}
							}
							if deal.MetadataPath != "" {
								err := storageDealPorcess.fs.Delete(deal.MetadataPath)
								if err != nil {
									log.Warnf("deleting piece at path %s: %w", deal.MetadataPath, err)
								}
							}

							if deal.InboundCAR != "" {
								if err := storageDealPorcess.TerminateBlockstore(deal.ProposalCid, deal.InboundCAR); err != nil {
									log.Warnf("failed to cleanup blockstore, car_path=%s: %s", deal.InboundCAR, err)
								}
							}

							deal.State = storagemarket.StorageDealActive // WaitForDealCompletion
							// At this point we have all the data so we can unprotect the connection
							storageDealPorcess.peerTagger.UntagPeer(deal.Client, deal.ProposalCid.String())

							node := storageDealPorcess.spn

							// Called when the deal expires
							expiredCb := func(err error) {
								if err != nil {
									storageDealPorcess.HandleError(deal, xerrors.Errorf("deal expiration err: %w", err))
								} else {
									deal.State = storagemarket.StorageDealExpired
								}
							}

							// Called when the deal is slashed
							slashedCb := func(slashEpoch abi.ChainEpoch, err error) {
								if err != nil {
									storageDealPorcess.HandleError(deal, xerrors.Errorf("deal slashing err: %w", err))
								} else {
									deal.SlashEpoch = slashEpoch
									deal.State = storagemarket.StorageDealSlashed
								}
							}

							if err := node.OnDealExpiredOrSlashed(ctx, deal.DealID, expiredCb, slashedCb); err != nil {
								storageDealPorcess.HandleError(deal, err)
							}
						}
					}

					err := node.OnDealSectorCommitted(ctx, deal.Proposal.Provider, deal.DealID, deal.SectorNumber, deal.Proposal, deal.PublishCid, cbDSC)
					if err != nil {
						storageDealPorcess.HandleError(deal, err)
					}
				}
			}
		}

		go func() {
			err := storageDealPorcess.spn.OnDealSectorPreCommitted(ctx, deal.Proposal.Provider, deal.DealID, deal.Proposal, deal.PublishCid, cb)
			if err != nil {
				storageDealPorcess.HandleError(deal, err)
			}
		}()
	}

	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) savePieceMetadata(deal *storagemarket.MinerDeal) error {

	var blockLocations map[cid.Cid]piecestore.BlockLocation
	if deal.MetadataPath != filestore.Path("") {
		var err error
		blockLocations, err = providerutils.LoadBlockLocations(storageDealPorcess.fs, deal.MetadataPath)
		if err != nil {
			return xerrors.Errorf("failed to load block locations: %w", err)
		}
	} else {
		blockLocations = map[cid.Cid]piecestore.BlockLocation{
			deal.Ref.Root: {},
		}
	}

	if err := storageDealPorcess.pieceStore.AddPieceBlockLocations(deal.Proposal.PieceCID, blockLocations); err != nil {
		return xerrors.Errorf("failed to add piece block locations: %s", err)
	}

	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) savePieceFile(ctx context.Context, deal *storagemarket.MinerDeal, reader io.Reader, payloadSize uint64) error {
	// because we use the PadReader directly during AP we need to produce the
	// correct amount of zeroes
	// (alternative would be to keep precise track of sector offsets for each
	// piece which is just too much work for a seldom used feature)
	unPadPieceSize := deal.Proposal.PieceSize.Unpadded()
	paddedReader, err := padreader.NewInflator(reader, payloadSize, deal.Proposal.PieceSize.Unpadded())
	if err != nil {
		return err
	}

	pieceCid := deal.ClientDealProposal.Proposal.PieceCID
	has, err := storageDealPorcess.storage.Has(pieceCid.String())
	if err != nil {
		return xerrors.Errorf("failed to get piece cid data %w", err)
	}

	if !has {
		wLen, err := storageDealPorcess.storage.SaveTo(ctx, pieceCid.String(), paddedReader)
		if err != nil {
			return err
		}
		if wLen != int64(unPadPieceSize) {
			return xerrors.Errorf("save piece expect len %d but got %d", unPadPieceSize, wLen)
		}
		log.Infof("success to write file %s to piece storage", pieceCid)
	}
	return nil
}

func (storageDealPorcess *StorageDealProcessImpl) SendSignedResponse(ctx context.Context, resp *network.Response) error {
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

// StorageDealRejecting(RejectDeal)->StorageDealFailing(FailDeal)
func (storageDealPorcess *StorageDealProcessImpl) HandleReject(deal *storagemarket.MinerDeal, event storagemarket.StorageDealStatus, err error) error {
	deal.State = event
	deal.Message = err.Error()

	err = storageDealPorcess.SendSignedResponse(context.TODO(), &network.Response{
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

	return storageDealPorcess.deals.SaveDeal(deal)
}

func (storageDealPorcess *StorageDealProcessImpl) HandleError(deal *storagemarket.MinerDeal, err error) error {
	deal.State = storagemarket.StorageDealFailing
	deal.Message = err.Error()

	log.Warnf("deal %s failed: %s", deal.ProposalCid, deal.Message)

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

	storageDealPorcess.releaseReservedFunds(context.TODO(), deal)

	return storageDealPorcess.deals.SaveDeal(deal)
}

func (storageDealPorcess *StorageDealProcessImpl) releaseReservedFunds(ctx context.Context, deal *storagemarket.MinerDeal) {
	if !deal.FundsReserved.Nil() && !deal.FundsReserved.IsZero() {
		err := storageDealPorcess.spn.ReleaseFunds(ctx, deal.Proposal.Provider, deal.FundsReserved)
		if err != nil {
			// nonfatal error
			log.Warnf("failed to release funds: %s", err)
		}

		deal.FundsReserved = big.Zero() // TODO: big.Subtract(deal.FundsReserved, fundsReleased)
	}
}

func (storageDealPorcess *StorageDealProcessImpl) SaveState(deal *storagemarket.MinerDeal, event storagemarket.StorageDealStatus) error {
	deal.State = event
	return storageDealPorcess.deals.SaveDeal(deal)
}

func (storageDealPorcess *StorageDealProcessImpl) ReadCAR(path string) (*carv2.Reader, error) {
	return carv2.OpenReader(path)
}

func (storageDealPorcess *StorageDealProcessImpl) FinalizeBlockstore(proposalCid cid.Cid) error {
	bs, err := storageDealPorcess.stores.Get(proposalCid.String())
	if err != nil {
		return xerrors.Errorf("failed to get read/write blockstore: %w", err)
	}

	if err := bs.Finalize(); err != nil {
		return xerrors.Errorf("failed to finalize read/write blockstore: %w", err)
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
