package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/config"
	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
)

type MsgMeta struct {
	Type string
	// Additional data related to what is signed. Should be verifiable with the
	// signed bytes (e.g. CID(Extra).Bytes() == toSign)
	Extra []byte
}

type IWalletClient interface {
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*vCrypto.Signature, error)
}

// *api.MessageImp and *gateway.WalletClient both implement IWalletClient, so injection will fail
type IWalletCli struct {
	IWalletClient
}

type WalletClient struct {
	Internal struct {
		WalletHas  func(ctx context.Context, addr address.Address) (bool, error)
		WalletSign func(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*vCrypto.Signature, error)
	}
}

func (walletClient *WalletClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return walletClient.Internal.WalletHas(ctx, addr)
}

func (walletClient *WalletClient) WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*vCrypto.Signature, error) {
	return walletClient.Internal.WalletSign(ctx, k, msg, meta)
}

func NewWalletClient(cfg *config.Gateway) (IWalletClient, jsonrpc.ClientCloser, error) {
	return newWalletClient(context.Background(), cfg.Token, cfg.Url)
}

func newWalletClient(ctx context.Context, token, url string) (*WalletClient, jsonrpc.ClientCloser, error) {
	apiInfo := apiinfo.NewAPIInfo(url, token)
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, err
	}

	walletClient := WalletClient{}
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Gateway", []interface{}{&walletClient.Internal}, apiInfo.AuthHeader())

	return &walletClient, closer, err
}
