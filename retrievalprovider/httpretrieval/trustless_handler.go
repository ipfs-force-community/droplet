package httpretrieval

import (
	"context"
	"net/url"
	"time"

	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/ipld/frisbii"
)

type trustlessHandler struct {
	*frisbii.HttpIpfs
}

func newTrustlessHandler(ctx context.Context, bs bstore.Blockstore, compressionLevel int) *trustlessHandler {
	lsys := storeutil.LinkSystemForBlockstore(bs)

	return &trustlessHandler{frisbii.NewHttpIpfs(ctx, lsys, frisbii.WithCompressionLevel(compressionLevel),
		frisbii.WithLogHandler(func(time time.Time, remoteAddr, method string, url url.URL, status int,
			duration time.Duration, bytes int, compressionRatio, userAgent, msg string) {
			log.Debugf("trustless handle %s %s %s %s %d %s %d %f %s %s", time, remoteAddr, method, url, status, duration,
				bytes, compressionRatio, userAgent, msg)
		}))}
}
