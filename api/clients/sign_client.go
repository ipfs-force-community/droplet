package clients

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/api/wallet"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/minermgr"
	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs-force-community/metrics"
	"go.uber.org/fx"
)

type ISinger interface {
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta types2.MsgMeta) (*vCrypto.Signature, error)
}

type SignerParams struct {
	fx.In
	SignerCfg *config.Signer
	Mgr       minermgr.IAddrMgr `optional:"true"`
}

func NewISignerClient(isServer bool) func(metrics.MetricsCtx, fx.Lifecycle, SignerParams) (ISinger, error) {
	return func(mctx metrics.MetricsCtx, lc fx.Lifecycle, params SignerParams) (ISinger, error) {
		var (
			cfg    = params.SignerCfg
			ctx    = metrics.LifecycleCtx(mctx, lc)
			signer ISinger
			closer jsonrpc.ClientCloser
			err    error
		)

		switch params.SignerCfg.SignerType {
		case config.SignerTypeWallet:
			signer, closer, err = wallet.DialIFullAPIRPC(ctx, cfg.Url, cfg.Token, nil)
		case config.SignerTypeGateway:
			if !isServer {
				return nil, fmt.Errorf("gateway signer not supported in client mode")
			}
			signer, closer, err = newGatewayWalletClient(context.Background(), params.Mgr, params.SignerCfg)
		default:
			return nil, fmt.Errorf("unsupport sign type %s", params.SignerCfg.SignerType)
		}

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				if closer != nil {
					closer()
				}
				return nil
			},
		})
		return signer, err
	}
}
