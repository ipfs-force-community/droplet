package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/app/client/apiface"
	"sync"
)

type INonceAssigner interface {
	AssignNonce(ctx context.Context, addr address.Address) (uint64, error)
}

type nonceAssigner struct {
	lk     sync.Mutex
	nonces map[address.Address]uint64
	full   apiface.FullNode
}

func newNonceAssign(full apiface.FullNode) *nonceAssigner {
	return &nonceAssigner{full: full, lk: sync.Mutex{}, nonces: map[address.Address]uint64{}}
}
func (nonceAssign *nonceAssigner) AssignNonce(ctx context.Context, addr address.Address) (uint64, error) {
	nonceAssign.lk.Lock()
	defer nonceAssign.lk.Unlock()

	mpoolNextNonce, err := nonceAssign.full.MpoolGetNonce(ctx, addr)
	if err != nil {
		return 0, nil
	}
	curNonce, ok := nonceAssign.nonces[addr]
	if ok {
		if mpoolNextNonce > curNonce {
			nonceAssign.nonces[addr] = mpoolNextNonce + 1
			return mpoolNextNonce, nil
		} else {
			nonceAssign.nonces[addr] = curNonce + 1
			return curNonce, nil
		}
	} else {
		nonceAssign.nonces[addr] = mpoolNextNonce + 1
		return mpoolNextNonce, nil
	}
}
