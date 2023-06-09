package signer

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"

	"github.com/ipfs-force-community/droplet/v2/config"

	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/venus-shared/api"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
)

type WalletClient struct {
	Internal struct {
		WalletHas  func(context.Context, address.Address) (bool, error)
		WalletSign func(context.Context, address.Address, []byte, vTypes.MsgMeta) (*vCrypto.Signature, error)
	}
}

func (walletClient *WalletClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return walletClient.Internal.WalletHas(ctx, addr)
}

func (walletClient *WalletClient) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error) {
	return walletClient.Internal.WalletSign(ctx, addr, msg, meta)
}

func newWalletClient(ctx context.Context, nodeCfg *config.Signer) (ISigner, jsonrpc.ClientCloser, error) {
	apiInfo := api.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	walletClient := WalletClient{}
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", []interface{}{&walletClient.Internal}, apiInfo.AuthHeader())

	return &walletClient, closer, err
}
