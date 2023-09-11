package utils

import (
	"io"

	"github.com/filecoin-project/go-commp-utils/ffiwrapper"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

func GeneratePieceCommitment(rt abi.RegisteredSealProof, rd io.Reader, pieceSize uint64) (cid.Cid, error) {
	paddedReader, paddedSize := padreader.New(rd, pieceSize)
	commitment, err := ffiwrapper.GeneratePieceCIDFromFile(rt, paddedReader, paddedSize)
	if err != nil {
		return cid.Undef, err
	}
	return commitment, nil
}

func GeneratePieceCommP(rt abi.RegisteredSealProof, rd io.Reader, carSize, pieceSize uint64) (cid.Cid, error) {
	pieceCid, err := GeneratePieceCommitment(rt, rd, carSize)
	if err != nil {
		return cid.Cid{}, nil
	}
	if carSizePadded := padreader.PaddedSize(carSize).Padded(); uint64(carSizePadded) < pieceSize {
		// need to pad up!
		rawPaddedCommp, err := commp.PadCommP(
			// we know how long a pieceCid "hash" is, just blindly extract the trailing 32 bytes
			pieceCid.Hash()[len(pieceCid.Hash())-32:],
			uint64(carSizePadded),
			pieceSize,
		)
		if err != nil {
			return cid.Undef, err
		}
		pieceCid, _ = commcid.DataCommitmentV1ToCID(rawPaddedCommp)
	}

	return pieceCid, nil
}
