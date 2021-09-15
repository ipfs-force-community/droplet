package sealer

import (
	"context"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/piece"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"io"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"

	"github.com/filecoin-project/lotus/extern/sector-storage/fr32"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
)

type PieceProvider interface {
	// ReadPiece is used to read an Unsealed piece at the given offset and of the given size from a Sector
	ReadPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, ticket abi.SealRandomness, unsealed cid.Cid) (io.ReadCloser, bool, error)
	IsUnsealed(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error)
}

var _ PieceProvider = &pieceProvider{}

type pieceProvider struct {
	pieceStorage piece.IPieceStorage
	exPieceStore piece.ExtendPieceStore
	miner        clients.IStorageMiner
}

func NewPieceProvider(miner clients.IStorageMiner, pieceStorage piece.IPieceStorage, exPieceStore piece.ExtendPieceStore) PieceProvider {
	return &pieceProvider{
		miner:        miner,
		pieceStorage: pieceStorage,
		exPieceStore: exPieceStore,
	}
}

// IsUnsealed checks if we have the unsealed piece at the given offset in an already
// existing unsealed file either locally or on any of the workers.
func (p *pieceProvider) IsUnsealed(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error) {
	if err := offset.Valid(); err != nil {
		return false, xerrors.Errorf("offset is not valid: %w", err)
	}
	if err := size.Validate(); err != nil {
		return false, xerrors.Errorf("size is not a valid piece size: %w", err)
	}

	ctxLock, cancel := context.WithCancel(ctx)
	defer cancel()

	dealInfo, err := p.exPieceStore.GetDealByPosition(ctx, sector.ID, abi.PaddedPieceSize(offset.Padded()))
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

	return p.miner.IsUnsealed(ctxLock, sector, offset, size)
}

// ReadPiece is used to read an Unsealed piece at the given offset and of the given size from a Sector
// If an Unsealed sector file exists with the Piece Unsealed in it, we'll use that for the read.
// Otherwise, we will Unseal a Sealed sector file for the given sector and read the Unsealed piece from it.
// If we do NOT have an existing unsealed file  containing the given piece thus causing us to schedule an Unseal,
// the returned boolean parameter will be set to true.
// If we have an existing unsealed file containing the given piece, the returned boolean will be set to false.
func (p *pieceProvider) ReadPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, ticket abi.SealRandomness, unsealed cid.Cid) (io.ReadCloser, bool, error) {
	//read directly from local piece store need piece cid
	if err := offset.Valid(); err != nil {
		return nil, false, xerrors.Errorf("offset is not valid: %w", err)
	}
	if err := size.Validate(); err != nil {
		return nil, false, xerrors.Errorf("size is not a valid piece size: %w", err)
	}

	dealInfo, err := p.exPieceStore.GetDealByPosition(ctx, sector.ID, abi.PaddedPieceSize(offset.Padded()))
	if err != nil {
		log.Errorf("did not get deal info by position;sector=%+v, err:%s", sector.ID, err)
		return nil, false, err
	}
	pieceCid := dealInfo.Proposal.PieceCID
	has, err := p.pieceStorage.Has(pieceCid.String())
	if err != nil {
		log.Errorf("did not check piece file in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
		return nil, false, err
	}

	var uns bool
	var r io.ReadCloser
	if has {
		r, err = p.pieceStorage.Read(ctx, pieceCid.String())
		if err != nil {
			log.Errorf("unable to read piece in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
			return nil, false, err
		}
		uns = true
		return r, true, err
	} else {

		//todo check deal status
		/*	if dealInfo.Status != "Proving" {
			log.Errorf("try read a unsealed sector ;sector=%+v, piececid=%s state=%s err:%s", sector.ID, pieceCid, dealInfo.Status, err)
			return nil, false, err
		}*/

		// a nil reader means that none of the workers has an unsealed sector file
		// containing the unsealed piece.
		// we now need to unseal a sealed sector file for the given sector to read the unsealed piece from it.
		commd := &unsealed
		if unsealed == cid.Undef {
			commd = nil
		}

		//todo how to tell sealer to start unseal work and async to check task completed
		if err := p.miner.SectorsUnsealPiece(ctx, sector, offset, size, ticket, commd); err != nil {
			log.Errorf("failed to SectorsUnsealPiece: %s", err)
			return nil, false, xerrors.Errorf("unsealing piece: %w", err)
		}

		//todo how to store data piece not completed piece
		log.Debugf("unsealed a sector file to read the piece, sector=%+v, offset=%d, size=%d", sector, offset, size)
		// move piece to storage
		r, err = p.pieceStorage.Read(ctx, pieceCid.String())
		if err != nil {
			log.Errorf("unable to read piece in piece storage;sector=%+v, piececid=%s err:%s", sector.ID, pieceCid, err)
			return nil, false, err
		}
		uns = false
	}

	upr, err := fr32.NewUnpadReader(r, size.Padded())
	if err != nil {
		return nil, false, xerrors.Errorf("creating unpadded reader: %w", err)
	}

	log.Debugf("returning reader to read unsealed piece, sector=%+v, offset=%d, size=%d", sector, offset, size)

	return &funcCloser{
		Reader: upr,
		close: func() error {
			err = r.Close()
			return err
		},
	}, uns, nil
}

type funcCloser struct {
	io.Reader
	close func() error
}

func (fc *funcCloser) Close() error { return fc.close() }
