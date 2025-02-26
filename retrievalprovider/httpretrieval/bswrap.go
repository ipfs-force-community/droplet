package httpretrieval

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/hashicorp/go-multierror"
	bstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multihash"
)

var errNotSupported = errors.New("not supported")

var _ bstore.Blockstore = (*bsWrap)(nil)

type bsWrap struct {
	dagStoreWrapper stores.DAGStoreWrapper
}

func newBSWrap(_ context.Context, dagStoreWrapper stores.DAGStoreWrapper) *bsWrap {
	return &bsWrap{
		dagStoreWrapper: dagStoreWrapper,
	}
}

func (bs *bsWrap) Has(ctx context.Context, blockCID cid.Cid) (bool, error) {
	pieces, err := bs.dagStoreWrapper.GetPiecesContainingBlock(blockCID)
	if err != nil {
		return false, err
	}

	return len(pieces) > 0, nil
}

func (bs *bsWrap) Get(ctx context.Context, blockCID cid.Cid) (blocks.Block, error) {
	pieces, err := bs.dagStoreWrapper.GetPiecesContainingBlock(blockCID)
	log.Debugf("bsWrap get %s %v", blockCID, pieces)

	// Check if it's an identity cid, if it is, return its digest
	if err != nil {
		digest, ok, iderr := isIdentity(blockCID)
		if iderr == nil && ok {
			return blocks.NewBlockWithCid(digest, blockCID)
		}
		return nil, fmt.Errorf("getting pieces containing cid %s: %w", blockCID, err)
	}

	if len(pieces) == 0 {
		return nil, fmt.Errorf("no pieces with cid %s found", blockCID)
	}

	// Get a reader over one of the pieces and extract the block
	var merr error
	for i, pieceCid := range pieces {
		blk, err := func() (blocks.Block, error) {
			// Get a reader over the piece data
			reader, err := bs.dagStoreWrapper.LoadShard(ctx, pieceCid)
			if err != nil {
				return nil, fmt.Errorf("getting piece reader: %w", err)
			}
			defer reader.Close() // nolint:errcheck

			return reader.Get(ctx, blockCID)
		}()
		if err != nil {
			if i < 3 {
				merr = multierror.Append(merr, err)
			}
			continue
		}
		return blk, nil
	}

	return nil, merr
}

func (bs *bsWrap) GetSize(ctx context.Context, blockCID cid.Cid) (int, error) {
	// Get the pieces that contain the cid
	pieces, err := bs.dagStoreWrapper.GetPiecesContainingBlock(blockCID)
	if err != nil {
		return 0, fmt.Errorf("getting pieces containing cid %s: %w", blockCID, err)
	}
	if len(pieces) == 0 {
		// We must return ipld ErrNotFound here because that's the only type
		// that bitswap interprets as a not found error. All other error types
		// are treated as general errors.
		return 0, format.ErrNotFound{Cid: blockCID}
	}

	var merr error

	// Iterate over all pieces in case the sector containing the first piece with the Block
	// is not unsealed
	for _, pieceCid := range pieces {
		reader, err := bs.dagStoreWrapper.LoadShard(ctx, pieceCid)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("getting piece reader: %w", err))
			continue
		}
		defer reader.Close() // nolint:errcheck

		size, err := reader.GetSize(ctx, blockCID)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("getting size of cid %s in piece %s: %w", blockCID, pieceCid, err))
			continue
		}

		return size, nil
	}

	return 0, merr
}

func (bs *bsWrap) Put(context.Context, blocks.Block) error {
	return errNotSupported
}

func (bs *bsWrap) PutMany(context.Context, []blocks.Block) error {
	return errNotSupported
}

func (bs *bsWrap) DeleteBlock(context.Context, cid.Cid) error {
	return errNotSupported
}

func (bs *bsWrap) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, errNotSupported
}

func (bs *bsWrap) HashOnRead(enabled bool) {}

func isIdentity(c cid.Cid) (digest []byte, ok bool, err error) {
	dmh, err := multihash.Decode(c.Hash())
	if err != nil {
		return nil, false, err
	}
	ok = dmh.Code == multihash.IDENTITY
	digest = dmh.Digest
	return digest, ok, nil
}
