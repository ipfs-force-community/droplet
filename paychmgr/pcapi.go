package paychmgr

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// paychDependencyAPI defines the API methods needed by the payment channel manager
type paychDependencyAPI interface {
	StateAccountKey(context.Context, address.Address, types.TipSetKey) (address.Address, error)
	WaitMsg(ctx context.Context, msg cid.Cid, confidence uint64) (*types.MsgLookup, error)
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error)
	StateNetworkVersion(context.Context, types.TipSetKey) (network.Version, error)
	PushMessage(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec) (cid.Cid, error)
}

type IMessagePush interface {
	PushMessage(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec) (cid.Cid, error)
	WaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)
}

type IChainInfo interface {
	StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error)
	StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error)
}

type IWalletAPI interface {
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error)
}
type pcAPI struct {
	mpAPI        IMessagePush
	chainInfoAPI IChainInfo
	walletAPI    IWalletAPI
}

func newPaychDependencyAPI(mpAPI IMessagePush, c IChainInfo, w IWalletAPI) paychDependencyAPI {
	return &pcAPI{mpAPI: mpAPI, chainInfoAPI: c, walletAPI: w}
}

func (o *pcAPI) StateAccountKey(ctx context.Context, address address.Address, tsk types.TipSetKey) (address.Address, error) {
	return o.chainInfoAPI.StateAccountKey(ctx, address, tsk)
}
func (o *pcAPI) WaitMsg(ctx context.Context, msg cid.Cid, confidence uint64) (*types.MsgLookup, error) {
	return o.mpAPI.WaitMsg(ctx, msg, confidence, constants.LookbackNoLimit, true)
}
func (o *pcAPI) PushMessage(ctx context.Context, msg *types.Message, msgMeta *types.MessageSendSpec) (cid.Cid, error) {
	return o.mpAPI.PushMessage(ctx, msg, msgMeta)
}
func (o *pcAPI) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return o.walletAPI.WalletHas(ctx, addr)
}
func (o *pcAPI) WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error) {
	return o.walletAPI.WalletSign(ctx, k, msg, types.MsgMeta{Type: types.MTSignedVoucher})
}
func (o *pcAPI) StateNetworkVersion(ctx context.Context, ts types.TipSetKey) (network.Version, error) {
	return o.chainInfoAPI.StateNetworkVersion(ctx, ts)
}
