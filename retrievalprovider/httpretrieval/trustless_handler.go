package httpretrieval

import (
	"context"

	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/ipld/frisbii"
)

type trustlessHandler struct {
	*frisbii.HttpIpfs
}

func newTrustlessHandler(ctx context.Context, bs bstore.Blockstore, compressionLevel int) *trustlessHandler {
	lsys := storeutil.LinkSystemForBlockstore(bs)

	return &trustlessHandler{frisbii.NewHttpIpfs(ctx, lsys, frisbii.WithCompressionLevel(compressionLevel))}
}
