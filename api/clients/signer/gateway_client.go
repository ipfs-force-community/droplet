package signer

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"

	"github.com/filecoin-project/venus-auth/jwtclient"

	"github.com/filecoin-project/venus-market/v2/config"

	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/venus-shared/api"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
)

type GatewayClient struct {
	Internal struct {
		WalletHas  func(context.Context, address.Address, []string) (bool, error)
		WalletSign func(context.Context, address.Address, []string, []byte, vTypes.MsgMeta) (*vCrypto.Signature, error)
	}
}

func (lnw *GatewayClient) WalletHas(ctx context.Context, addr address.Address, accounts []string) (bool, error) {
	return lnw.Internal.WalletHas(ctx, addr, accounts)
}

func (lnw *GatewayClient) WalletSign(ctx context.Context, addr address.Address, accounts []string, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error) {
	return lnw.Internal.WalletSign(ctx, addr, accounts, msg, meta)
}

type WrapperGatewayClient struct {
	gatewayClient *GatewayClient
	authClient    jwtclient.IAuthClient
}

func (w *WrapperGatewayClient) getAccountsOfSigner(ctx context.Context, addr address.Address) ([]string, error) {
	users, err := w.authClient.GetUserBySigner(ctx, addr)
	if err != nil {
		return nil, err
	}

	accounts := make([]string, 0, len(users))
	for _, user := range users {
		accounts = append(accounts, user.Name)
	}

	return accounts, nil
}

func (w *WrapperGatewayClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	accounts, err := w.getAccountsOfSigner(ctx, addr)
	if err != nil {
		return false, err
	}
	return w.gatewayClient.WalletHas(ctx, addr, accounts)
}

func (w *WrapperGatewayClient) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error) {
	accounts, err := w.getAccountsOfSigner(ctx, addr)
	if err != nil {
		return nil, err
	}
	return w.gatewayClient.WalletSign(ctx, addr, accounts, msg, meta)
}

func newGatewayWalletClient(ctx context.Context, nodeCfg *config.Signer, authClient jwtclient.IAuthClient) (ISigner, jsonrpc.ClientCloser, error) {
	apiInfo := api.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	dialAddr, err := apiInfo.DialArgs("v2")
	if err != nil {
		return nil, nil, err
	}

	gatewayClient := GatewayClient{}
	closer, err := jsonrpc.NewMergeClient(ctx, dialAddr, "Gateway", api.GetInternalStructs(&gatewayClient), apiInfo.AuthHeader())
	if err != nil {
		return nil, nil, err
	}

	return &WrapperGatewayClient{gatewayClient: &gatewayClient, authClient: authClient}, closer, err
}
