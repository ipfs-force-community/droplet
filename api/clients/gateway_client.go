package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/minermgr"
	"github.com/filecoin-project/venus-messager/gateway"
	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	wallet2 "github.com/ipfs-force-community/venus-gateway/types/wallet"
	"unsafe"
)

func newGatewayWalletClient(mctx metrics.MetricsCtx, mgr minermgr.IMinerMgr, nodeCfg *config.Signer) (ISinger, jsonrpc.ClientCloser, error) {
	info := apiinfo.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	dialAddr, err := info.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	var client gateway.WalletClient
	closer, err := jsonrpc.NewClient(mctx, dialAddr, "Gateway", client, info.AuthHeader())

	return &GatewayClient{
		innerClient: client,
		importMgr:   mgr,
	}, closer, err
}

type GatewayClient struct {
	innerClient gateway.WalletClient
	importMgr   minermgr.IMinerMgr
}

func (gatewayClient *GatewayClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	account, err := gatewayClient.importMgr.GetAccount(ctx, addr)
	if err != nil {
		return false, err
	}
	return gatewayClient.innerClient.WalletHas(ctx, account, addr)
}

func (gatewayClient *GatewayClient) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta wallet.MsgMeta) (*vCrypto.Signature, error) {
	account, err := gatewayClient.importMgr.GetAccount(ctx, addr)
	if err != nil {
		return nil, err
	}

	return gatewayClient.innerClient.WalletSign(ctx, account, addr, msg, *(*wallet2.MsgMeta)(unsafe.Pointer(&meta)))
}
