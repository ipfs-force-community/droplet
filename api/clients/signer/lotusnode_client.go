package signer

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"

	"github.com/ipfs-force-community/venus-common-utils/apiinfo"

	"github.com/filecoin-project/venus-market/v2/config"

	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
)

type LotusnodeClient struct {
	Internal struct {
		WalletHas  func(context.Context, address.Address) (bool, error)
		WalletSign func(context.Context, address.Address, []byte) (*vCrypto.Signature, error)
	}
}

func (lnw *LotusnodeClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return lnw.Internal.WalletHas(ctx, addr)
}

func (lnw *LotusnodeClient) WalletSign(ctx context.Context, addr address.Address, msg []byte) (*vCrypto.Signature, error) {
	return lnw.Internal.WalletSign(ctx, addr, msg)
}

type WrapperLotusnodeClient struct {
	lotusnodeClient *LotusnodeClient
}

func (w *WrapperLotusnodeClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return w.lotusnodeClient.WalletHas(ctx, addr)
}

func (w *WrapperLotusnodeClient) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error) {
	return w.lotusnodeClient.WalletSign(ctx, addr, msg)
}

func newLotusnodeClient(ctx context.Context, nodeCfg *config.Signer) (ISigner, jsonrpc.ClientCloser, error) {
	apiInfo := apiinfo.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}

	lotusnodeClient := LotusnodeClient{}
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", []interface{}{&lotusnodeClient.Internal}, apiInfo.AuthHeader())

	return &WrapperLotusnodeClient{lotusnodeClient: &lotusnodeClient}, closer, err
}
