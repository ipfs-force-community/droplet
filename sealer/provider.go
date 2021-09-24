package sealer

import (
	"context"
	"github.com/filecoin-project/go-address"
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/piece"
	"github.com/filecoin-project/venus-market/types"
	types2 "github.com/ipfs-force-community/venus-common-utils/types"
	"golang.org/x/xerrors"
	"io"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
)

type PieceProvider interface {
	// ReadPiece is used to read an Unsealed piece at the given offset and of the given size from a Sector
	ReadPiece(ctx context.Context, sector storage.SectorRef, offset types2.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (io.ReadCloser, bool, error)
	IsUnsealed(ctx context.Context, sector storage.SectorRef, offset types2.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error)
}

var _ PieceProvider = &pieceProvider{}

type pieceProvider struct {
	pieceStorage     piece.IPieceStorage
	exPieceStore     piece.ExtendPieceStore
	miner            clients2.MarketRequestEvent
	maddr            types.MinerAddress
	pieceStrorageCfg *config.PieceStorageString
}

func NewPieceProvider(miner clients2.MarketRequestEvent, maddr types.MinerAddress, pieceStrorageCfg *config.PieceStorageString, pieceStorage piece.IPieceStorage, exPieceStore piece.ExtendPieceStore) PieceProvider {
	return &pieceProvider{
		miner:            miner,
		pieceStorage:     pieceStorage,
		exPieceStore:     exPieceStore,
		maddr:            maddr,
		pieceStrorageCfg: pieceStrorageCfg,
	}
}

// IsUnsealed checks if we have the unsealed piece at the given offset in an already
// existing unsealed file either locally or on any of the workers.
func (p *pieceProvider) IsUnsealed(ctx context.Context, sector storage.SectorRef, offset types2.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error) {
	if err := offset.Valid(); err != nil {
		return false, xerrors.Errorf("offset is not valid: %w", err)
	}
	if err := size.Validate(); err != nil {
		return false, xerrors.Errorf("size is not a valid piece size: %w", err)
	}

	ctxLock, cancel := context.WithCancel(ctx)
	defer cancel()

	dealInfo, err := p.exPieceStore.GetDealByPosition(ctx, sector.ID, abi.PaddedPieceSize(offset.Padded()), size.Padded())
	if err != nil {
		log.Errorf("did not get deal info by position;sector=%+v, err:%s", sector.ID, err)
		return false, err
	}
	pieceCid := dealInfo.Proposal.PieceCID
	has, err := p.pieceStorage.Has(pieceCid.String())
	if err != nil {
		log.Errorf("did not check piece file in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
		return false, err
	}

	if has {
		return true, nil
	}

	return p.miner.IsUnsealed(ctxLock, address.Address(p.maddr), pieceCid, sector, offset.Padded(), size.Padded())
}

// ReadPiece is used to read an Unsealed piece at the given offset and of the given size from a Sector
// If an Unsealed sector file exists with the Piece Unsealed in it, we'll use that for the read.
// Otherwise, we will Unseal a Sealed sector file for the given sector and read the Unsealed piece from it.
// If we do NOT have an existing unsealed file  containing the given piece thus causing us to schedule an Unseal,
// the returned boolean parameter will be set to true.
// If we have an existing unsealed file containing the given piece, the returned boolean will be set to false.
func (p *pieceProvider) ReadPiece(ctx context.Context, sector storage.SectorRef, offset types2.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (io.ReadCloser, bool, error) {
	//read directly from local piece store need piece cid
	if err := offset.Valid(); err != nil {
		return nil, false, xerrors.Errorf("offset is not valid: %w", err)
	}
	if err := size.Validate(); err != nil {
		return nil, false, xerrors.Errorf("size is not a valid piece size: %w", err)
	}

	dealInfo, err := p.exPieceStore.GetDealByPosition(ctx, sector.ID, abi.PaddedPieceSize(offset.Padded()), size.Padded())
	if err != nil {
		log.Errorf("did not get deal info by position;sector=%+v, err:%s", sector.ID, err)
		return nil, false, err
	}
	pieceCid := dealInfo.Proposal.PieceCID
	pieceOffset := abi.UnpaddedPieceSize(offset) - dealInfo.Offset.Unpadded()
	has, err := p.pieceStorage.Has(pieceCid.String())
	if err != nil {
		log.Errorf("did not check piece file in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
		return nil, false, err
	}

	if has {
		r, err := p.pieceStorage.ReadSize(ctx, pieceCid.String(), pieceOffset, size)
		if err != nil {
			log.Errorf("unable to read piece in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
			return nil, false, err
		}
		return r, true, err
	} else {

		r, err := p.unsealPiece(ctx, dealInfo, sector, offset, size)
		if err != nil {
			return nil, false, xerrors.Errorf("unseal piece %w", err)
		}
		return r, false, nil
	}
}

func (p *pieceProvider) unsealPiece(ctx context.Context, dealInfo *piece.DealInfo, sector storage.SectorRef, offset types2.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (io.ReadCloser, error) {
	pieceCid := dealInfo.Proposal.PieceCID
	pieceOffset := abi.UnpaddedPieceSize(offset) - dealInfo.Offset.Unpadded()
	if err := p.miner.SectorsUnsealPiece(ctx, address.Address(p.maddr), pieceCid, sector, offset.Padded(), size.Padded(), string(*p.pieceStrorageCfg)); err != nil {
		log.Errorf("failed to SectorsUnsealPiece: %s", err)
		return nil, xerrors.Errorf("unsealing piece: %w", err)
	}

	//todo config
	ctx, _ = context.WithTimeout(ctx, time.Hour*6)
	tm := time.NewTimer(time.Second * 30)

	for {
		select {
		case <-tm.C:
			has, err := p.pieceStorage.Has(pieceCid.String())
			if err != nil {
				return nil, xerrors.Errorf("unable to check piece in piece stroage %w", err)
			}
			if has {
				goto LOOP
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
LOOP:
	//todo how to store data piece not completed piece
	log.Debugf("unsealed a sector file to read the piece, sector=%+v, offset=%d, size=%d", sector, offset, size)
	// move piece to storage
	r, err := p.pieceStorage.ReadSize(ctx, pieceCid.String(), pieceOffset, size)
	if err != nil {
		log.Errorf("unable to read piece in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
		return nil, err
	}
	return r, err
}
