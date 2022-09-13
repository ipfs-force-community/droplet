package clients

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"

	"github.com/filecoin-project/venus-market/v2/config"

	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	gwAPI "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	sharedTypes "github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs-force-community/metrics"
)

func newGatewayWalletClient(mctx metrics.MetricsCtx, nodeCfg *config.Signer) (ISinger, jsonrpc.ClientCloser, error) {
	client, closer, err := gwAPI.DialIGatewayRPC(mctx, nodeCfg.Url, nodeCfg.Token, nil)
	return &GatewayClient{
		innerClient: client,
	}, closer, err
}

type GatewayClient struct {
	innerClient gwAPI.IWalletClient
}

func (gatewayClient *GatewayClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return gatewayClient.innerClient.WalletHas(ctx, addr)
}

func (gatewayClient *GatewayClient) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta sharedTypes.MsgMeta) (*vCrypto.Signature, error) {
	return gatewayClient.innerClient.WalletSign(ctx, addr, msg, meta)
}
