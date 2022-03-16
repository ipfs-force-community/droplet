package clients

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-address"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
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

//AssignNonce assign next nonce for address, in solo mode, should use a separate address for market message, should save nonce
//when only connect one daemon, MpoolGetNonce works well, but may have conflict nonce if use multiple daemon behind proxy
//todo save assgined nonce in local database
func (nonceAssign *nonceAssigner) AssignNonce(ctx context.Context, addr address.Address) (uint64, error) {
	nonceAssign.lk.Lock()
	defer nonceAssign.lk.Unlock()

	return nonceAssign.full.MpoolGetNonce(ctx, addr)
}
