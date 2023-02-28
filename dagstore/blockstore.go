package dagstore

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-libipfs/blocks"

	"github.com/filecoin-project/dagstore"
)

// Blockstore promotes a dagstore.ReadBlockstore to a full closeable Blockstore,
// stubbing out the write methods with erroring implementations.
type Blockstore struct {
	dagstore.ReadBlockstore
	io.Closer
}

var _ bstore.Blockstore = (*Blockstore)(nil)

func (b *Blockstore) DeleteBlock(context.Context, cid.Cid) error {
	return fmt.Errorf("DeleteBlock called but not implemented")
}

func (b *Blockstore) Put(context.Context, blocks.Block) error {
	return fmt.Errorf("Put called but not implemented")
}

func (b *Blockstore) PutMany(context.Context, []blocks.Block) error {
	return fmt.Errorf("PutMany called but not implemented")
}
