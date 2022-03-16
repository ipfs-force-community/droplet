package paychmgr

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type WalletAdapter struct {
}

func (w WalletAdapter) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	panic("implement me")
}

func (w WalletAdapter) WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error) {

	panic("implement me")
}
