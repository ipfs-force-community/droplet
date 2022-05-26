package dagstore

import (
	"context"
	"fmt"
	"io"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/dagstore"
)

// Blockstore promotes a dagstore.ReadBlockstore to a full closeable Blockstore,
// stubbing out the write methods with erroring implementations.
type Blockstore struct {
	dagstore.ReadBlockstore
	io.Closer
}

var _ bstore.Blockstore = (*Blockstore)(nil)

func (b *Blockstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	return fmt.Errorf("deleteBlock called but not implemented")
}

func (b *Blockstore) Put(ctx context.Context, block blocks.Block) error {
	return fmt.Errorf("put called but not implemented")
}

func (b *Blockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	return fmt.Errorf("putMany called but not implemented")
}
