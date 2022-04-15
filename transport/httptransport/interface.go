package httptransport

import (
	"context"

	"github.com/filecoin-project/venus-market/types"
)

type Transport interface {
	Execute(ctx context.Context, info *types.TransportInfo) (th Handler, err error)
}

type Handler interface {
	Sub() chan types.TransportEvent
	Close()
}
