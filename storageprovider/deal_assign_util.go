package storageprovider

import (
	"fmt"
	"math/bits"

	"github.com/filecoin-project/go-state-types/builtin/v8/market"

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
	space := abi.PaddedPieceSize(ssize)

	if err := space.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %d", errInvalidSpaceSize, space)
	}

	// 为了方便测试，将此过滤置于此位置
	// 如果为了考虑效率，且有合适的方式进行测试，则可以移动到前置逻辑中进行过滤
	// 确保订单在:
	//   1. (spec.StartEpoch, spec.EndEpoch)
	//   2. (0, spec.EndEpoch)
	//   3. (spec.StartEpoch, +inf)
	// 范围内
	if spec != nil && (spec.StartEpoch > 0 || spec.EndEpoch > 0) {
		picked := make([]*mtypes.DealInfoIncludePath, 0, len(deals))
		for di := range deals {
			deal := deals[di]
			if spec.StartEpoch > 0 && deal.DealProposal.StartEpoch <= spec.StartEpoch {
				continue
			}

			if spec.EndEpoch > 0 && deal.DealProposal.EndEpoch >= spec.EndEpoch {
				continue
			}

			picked = append(picked, deal)
		}

		deals = picked
	}

	if len(deals) > 0 && deals[0].PieceSize.Validate() != nil {
		return nil, fmt.Errorf("%w: first deal size: %d", errInvalidDealPieceSize, deals[0].PieceSize)
	}

	// 过滤掉太小的 deals
	if spec != nil && spec.MinPieceSize > 0 {
		limit := abi.PaddedPieceSize(spec.MinPieceSize)
		first := len(deals)

		// find the first deal index with piece size >= limit,
		// or all deals are too small
		for i := 0; i < len(deals); i++ {
			if deals[i].PieceSize >= limit {
				first = i
				break
			}
		}

		deals = deals[first:]
	}

	// 过滤掉太大的 deals
	if spec != nil && spec.MaxPieceSize > 0 {
		limit := abi.PaddedPieceSize(spec.MaxPieceSize)

		last := 0

		// find the last deal index with piece size <= limit,
		// or all deals are too large
		for i := len(deals); i > 0; i-- {
			if deals[i-1].PieceSize <= limit {
				last = i
				break
			}
		}

		deals = deals[:last]
	}

	// only the deals left
	dealCount := len(deals)
	if dealCount == 0 {
		return nil, nil
	}

	res := make([]*mtypes.DealInfoIncludePath, 0)
	di := 0
	checked := 0

	pickedDeals := 0
	pickedSpace := abi.PaddedPieceSize(0)

	var offset abi.UnpaddedPieceSize
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
		if spec != nil && spec.MaxPiece > 0 && pickedDeals >= spec.MaxPiece {
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
				DealProposal: market.DealProposal{
					PieceSize: nextPiece,
					PieceCID:  zerocomm.ZeroPieceCommitment(nextPiece.Unpadded()),
				},
			})

			space -= nextPiece
			offset += nextPiece.Unpadded()
			continue
		}

		deal.Offset = offset.Padded()
		res = append(res, deal)

		pickedDeals++
		pickedSpace += deal.PieceSize

		space -= deal.PieceSize
		offset += deal.PieceSize.Unpadded()
		di++
	}

	// no deals picked, we just do nothing here
	if len(res) == 0 {
		return nil, nil
	}

	// not enough deals
	if spec != nil && spec.MinPiece > 0 && pickedDeals < spec.MinPiece {
		return nil, nil
	}

	// not enough space for deals
	if spec != nil && spec.MinUsedSpace > 0 && uint64(pickedSpace) < spec.MinUsedSpace {
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
				DealProposal: market.DealProposal{
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
