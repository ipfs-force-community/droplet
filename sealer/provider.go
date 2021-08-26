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

	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
)

type PieceProvider interface {
	UnsealSector(ctx context.Context, sectorID abi.SectorNumber, offset abi.UnpaddedPieceSize, length abi.UnpaddedPieceSize) (io.ReadCloser, error)
	IsUnsealed(ctx context.Context, sectorID abi.SectorNumber, offset abi.UnpaddedPieceSize, length abi.UnpaddedPieceSize) (bool, error)
}

var _ PieceProvider = &pieceProvider{}

type pieceProvider struct {
	storage     piece.IPieceStorage
	uns         Unsealer
	minerClient clients.IStorageMiner //todo support multi
}

func (p *pieceProvider) UnsealSector(ctx context.Context, sectorID abi.SectorNumber, offset abi.UnpaddedPieceSize, length abi.UnpaddedPieceSize) (io.ReadCloser, error) {
	panic("implement me")
}

func (p *pieceProvider) IsUnsealed(ctx context.Context, sectorID abi.SectorNumber, offset abi.UnpaddedPieceSize, length abi.UnpaddedPieceSize) (bool, error) {
	if err := storiface.UnpaddedByteIndex(offset).Valid(); err != nil {
		return false, xerrors.Errorf("offset is not valid: %w", err)
	}
	if err := storiface.UnpaddedByteIndex(length).Valid(); err != nil {
		return false, xerrors.Errorf("size is not a valid piece size: %w", err)
	}

	ctxLock, cancel := context.WithCancel(ctx)
	defer cancel()

	//need piece to found
	sectorRef := storage.SectorRef{
		ID: abi.SectorID{
			Miner:  0,
			Number: sectorID,
		},
		ProofType: 0, //todo miner selector
	}

	return p.minerClient.IsUnsealed(ctxLock, sectorRef, storiface.UnpaddedByteIndex(offset), length)
}

func NewPieceProvider(storage piece.IPieceStorage, uns Unsealer, minerClient clients.IStorageMiner) PieceProvider {
	return &pieceProvider{
		storage:     storage,
		uns:         uns,
		minerClient: minerClient,
	}
}

// ReadPiece is used to read an Unsealed piece at the given offset and of the given size from a Sector
// If an Unsealed sector file exists with the Piece Unsealed in it, we'll use that for the read.
// Otherwise, we will Unseal a Sealed sector file for the given sector and read the Unsealed piece from it.
// If we do NOT have an existing unsealed file  containing the given piece thus causing us to schedule an Unseal,
// the returned boolean parameter will be set to true.
// If we have an existing unsealed file containing the given piece, the returned boolean will be set to false.
func (p *pieceProvider) ReadPiece(ctx context.Context, sector storage.SectorRef, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize, ticket abi.SealRandomness, unsealed cid.Cid) (io.ReadCloser, bool, error) {
	panic("to impl")
	/*if err := offset.Valid(); err != nil {
		return nil, false, xerrors.Errorf("offset is not valid: %w", err)
	}
	if err := size.Validate(); err != nil {
		return nil, false, xerrors.Errorf("size is not a valid piece size: %w", err)
	}

	r, unlock, err := p.tryReadUnsealedPiece(ctx, sector, offset, size)

	log.Debugf("result of first tryReadUnsealedPiece: r=%+v, err=%s", r, err)

	if xerrors.Is(err, storiface.ErrSectorNotFound) {
		log.Debugf("no unsealed sector file with unsealed piece, sector=%+v, offset=%d, size=%d", sector, offset, size)
		err = nil
	}
	if err != nil {
		log.Errorf("returning error from ReadPiece:%s", err)
		return nil, false, err
	}

	var uns bool

	if r == nil {
		// a nil reader means that none of the workers has an unsealed sector file
		// containing the unsealed piece.
		// we now need to unseal a sealed sector file for the given sector to read the unsealed piece from it.
		uns = true
		commd := &unsealed
		if unsealed == cid.Undef {
			commd = nil
		}
		if err := p.uns.SectorsUnsealPiece(ctx, sector, offset, size, ticket, commd); err != nil {
			log.Errorf("failed to SectorsUnsealPiece: %s", err)
			return nil, false, xerrors.Errorf("unsealing piece: %w", err)
		}

		log.Debugf("unsealed a sector file to read the piece, sector=%+v, offset=%d, size=%d", sector, offset, size)

		r, unlock, err = p.tryReadUnsealedPiece(ctx, sector, offset, size)
		if err != nil {
			log.Errorf("failed to tryReadUnsealedPiece after SectorsUnsealPiece: %s", err)
			return nil, true, xerrors.Errorf("read after unsealing: %w", err)
		}
		if r == nil {
			log.Errorf("got no reader after unsealing piece")
			return nil, true, xerrors.Errorf("got no reader after unsealing piece")
		}
		log.Debugf("got a reader to read unsealed piece, sector=%+v, offset=%d, size=%d", sector, offset, size)
	} else {
		log.Debugf("unsealed piece already exists, no need to unseal, sector=%+v, offset=%d, size=%d", sector, offset, size)
	}

	upr, err := fr32.NewUnpadReader(r, size.Padded())
	if err != nil {
		unlock()
		return nil, uns, xerrors.Errorf("creating unpadded reader: %w", err)
	}

	log.Debugf("returning reader to read unsealed piece, sector=%+v, offset=%d, size=%d", sector, offset, size)

	return &funcCloser{
		Reader: bufio.NewReaderSize(upr, 127),
		close: func() error {
			err = r.Close()
			unlock()
			return err
		},
	}, uns, nil*/
}

type funcCloser struct {
	io.Reader
	close func() error
}

func (fc *funcCloser) Close() error { return fc.close() }
