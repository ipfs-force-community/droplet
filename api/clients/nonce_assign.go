package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"sync"
)

type INonceAssigner interface {
	AssignNonce(ctx context.Context, addr address.Address) (uint64, error)
}

type nonceAssigner struct {
	lk   sync.Mutex
	full v1api.FullNode
}

func newNonceAssign(full v1api.FullNode) *nonceAssigner {
	return &nonceAssigner{full: full, lk: sync.Mutex{}}
}
func (nonceAssign *nonceAssigner) AssignNonce(ctx context.Context, addr address.Address) (uint64, error) {
	nonceAssign.lk.Lock()
	defer nonceAssign.lk.Unlock()

	return nonceAssign.full.MpoolGetNonce(ctx, addr)
}
