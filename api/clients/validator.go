package clients

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs-force-community/venus-gateway/validator"
)

type localMinerValidator struct{}

func (lmv *localMinerValidator) Validate(_ context.Context, _ address.Address) error {
	return nil
}

var _ validator.IAuthMinerValidator = (*localMinerValidator)(nil)
