package storageprovider

import (
	"fmt"
	market7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"math/bits"

	"github.com/filecoin-project/go-commp-utils/zerocomm"
	"github.com/filecoin-project/go-state-types/abi"
	mtypes "github.com/filecoin-project/venus/venus-shared/types/market"
)

var (
	errInvalidSpaceSize     = fmt.Errorf("invalid space size")
	errInvalidDealPieceSize = fmt.Errorf("invalid deal piece size")
	errDealsUnOrdered       = fmt.Errorf("deals un-ordered")
)

func pickAndAlign(deals []*mtypes.DealInfoIncludePath, ssize abi.SectorSize, spec *mtypes.GetDealSpec) ([]*mtypes.DealInfoIncludePath, error) {
	dealCount := len(deals)
	if dealCount == 0 {
		return nil, nil
	}

	space := abi.PaddedPieceSize(ssize)

	if err := space.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %d", errInvalidSpaceSize, space)
	}

	if psize := deals[0].PieceSize; psize.Validate() != nil {
		return nil, fmt.Errorf("%w: first deal size: %d", errInvalidDealPieceSize, psize)
	}

	var dealLimit *int
	var dealSizeLimit *abi.PaddedPieceSize
	if spec != nil {
		if spec.MaxPiece > 0 {
			dealLimit = &spec.MaxPiece
		}

		if spec.MaxPieceSize > 0 {
			limit := abi.PaddedPieceSize(spec.MaxPieceSize)
			dealSizeLimit = &limit
		}
	}

	res := make([]*mtypes.DealInfoIncludePath, 0)
	di := 0
	checked := 0

	for di < dealCount {
		deal := deals[di]
		if di != checked {
			if psize := deal.PieceSize; psize.Validate() != nil {
				return nil, fmt.Errorf("%w: #%d deal size: %d", errInvalidDealPieceSize, di, psize)
			}

			// deals unordered
			if di > 0 && deal.PieceSize < deals[di-1].PieceSize {
				return nil, errDealsUnOrdered
			}

			checked = di
		}

		// deal limit
		if dealLimit != nil && di >= *dealLimit {
			break
		}

		// deal size limit
		if dealSizeLimit != nil && deal.PieceSize > *dealSizeLimit {
			break
		}

		// not enough for next deal
		if deal.PieceSize > space {
			break
		}

		nextPiece := nextAlignedPiece(space)
		// next piece cantainer is not enough, we should put a zeroed-piece
		if deal.PieceSize > nextPiece {
			res = append(res, &mtypes.DealInfoIncludePath{
				DealProposal: market7.DealProposal{
					PieceSize: nextPiece,
					PieceCID:  zerocomm.ZeroPieceCommitment(nextPiece.Unpadded()),
				},
			})

			space -= nextPiece
			continue
		}

		res = append(res, deal)
		space -= deal.PieceSize
		di++
	}

	// no deals picked, we just do nothing here
	if len(res) == 0 {
		return nil, nil
	}

	// still have space left, we should fill in more zeroed-pieces
	if space > 0 {
		fillers, err := fillersFromRem(space)
		if err != nil {
			return nil, fmt.Errorf("get filler pieces for the remaining space %d: %w", space, err)
		}

		for _, fillSize := range fillers {
			res = append(res, &mtypes.DealInfoIncludePath{
				DealProposal: market7.DealProposal{
					PieceSize: fillSize,
					PieceCID:  zerocomm.ZeroPieceCommitment(fillSize.Unpadded()),
				},
			})
		}
	}

	return res, nil
}

func nextAlignedPiece(space abi.PaddedPieceSize) abi.PaddedPieceSize {
	return abi.PaddedPieceSize(1 << bits.TrailingZeros64(uint64(space)))
}

func fillersFromRem(in abi.PaddedPieceSize) ([]abi.PaddedPieceSize, error) {
	toFill := uint64(in)

	// We need to fill the sector with pieces that are powers of 2. Conveniently
	// computers store numbers in binary, which means we can look at 1s to get
	// all the piece sizes we need to fill the sector. It also means that number
	// of pieces is the number of 1s in the number of remaining bytes to fill
	out := make([]abi.PaddedPieceSize, bits.OnesCount64(toFill))
	for i := range out {
		// Extract the next lowest non-zero bit
		next := bits.TrailingZeros64(toFill)
		psize := uint64(1) << uint64(next)
		// e.g: if the number is 0b010100, psize will be 0b000100

		// set that bit to 0 by XORing it, so the next iteration looks at the
		// next bit
		toFill ^= psize

		// Add the piece size to the list of pieces we need to create
		out[i] = abi.PaddedPieceSize(psize)
	}

	return out, nil
}
