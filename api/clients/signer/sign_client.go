package signer

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"

	"github.com/ipfs-force-community/metrics"

	"github.com/ipfs-force-community/sophon-auth/jwtclient"

	"github.com/ipfs-force-community/droplet/v2/config"

	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
)

type ISigner interface {
	WalletHas(ctx context.Context, signerAddr address.Address) (bool, error)
	WalletSign(ctx context.Context, signerAddr address.Address, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error)
}

func NewISignerClient(isServer bool, authClient jwtclient.IAuthClient) func(metrics.MetricsCtx, *config.Signer) (ISigner, jsonrpc.ClientCloser, error) {
	return func(mCtx metrics.MetricsCtx, signerCfg *config.Signer) (ISigner, jsonrpc.ClientCloser, error) {
		var (
			signer ISigner
			closer jsonrpc.ClientCloser
			err    error
		)

		switch signerCfg.SignerType {
		// Sign with lotus node
		case config.SignerTypeLotusnode:
			signer, closer, err = newLotusnodeClient(mCtx, signerCfg)
		// Sign with lotus-wallet/venus-wallet/other wallet
		case config.SignerTypeWallet:
			signer, closer, err = newWalletClient(mCtx, signerCfg)
		// Signing through venus chain-service
		case config.SignerTypeGateway:
			if !isServer {
				return nil, nil, fmt.Errorf("signing through the sophon-gateway cannot be used for droplet-clientt")
			}
			signer, closer, err = newGatewayWalletClient(mCtx, signerCfg, authClient)
		default:
			return nil, nil, fmt.Errorf("unsupport signer type %s", signerCfg.SignerType)
		}

		return signer, closer, err
	}
}

func NewISignerClientWithLifecycle(isServer bool, authClient jwtclient.IAuthClient) func(metrics.MetricsCtx, fx.Lifecycle, *config.Signer) (ISigner, error) {
	return func(mc metrics.MetricsCtx, lc fx.Lifecycle, signerCfg *config.Signer) (ISigner, error) {
		signer, closer, err := NewISignerClient(isServer, authClient)(mc, signerCfg)
		if err != nil {
			return nil, err
		}

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				closer()
				return nil
			},
		})

		return signer, nil
	}
}
