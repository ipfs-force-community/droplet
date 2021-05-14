package client

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"net/http"
	"net/url"
)

type ChainAPI interface {
	StateNetworkVersion(context.Context, types.TipSetKey) (network.Version, error)
	ChainGetMessage(context.Context, cid.Cid) (*types.Message, error)
	ChainHead(context.Context) (*types.TipSet, error)
}

type PaychAPI interface {
	PaychVoucherAdd(context.Context, address.Address, *paych.SignedVoucher, []byte, types.BigInt) (types.BigInt, error)
	PaychAvailableFunds(ctx context.Context, ch address.Address) (*ChannelAvailableFunds, error)
	PaychGetWaitReady(context.Context, cid.Cid) (address.Address, error)
	PaychVoucherCreate(context.Context, address.Address, types.BigInt, uint64) (*VoucherCreateResult, error)
	PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error)
	PaychGet(ctx context.Context, from, to address.Address, amt types.BigInt) (*paychmgr.ChannelInfo, error)
}

type NodeClient interface {
	PaychAPI
	StateAPI
	ChainAPI
	MpoolAPI
}

type StateAPI interface {
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*chain.MsgLookup, error)
	StateMarketStorageDeal(ctx context.Context, dealId abi.DealID, tsk types.TipSetKey) (*chain.MarketDeal, error)
	StateLookupID(context.Context, address.Address, types.TipSetKey) (address.Address, error)
	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error)
	StateDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, tsk types.TipSetKey) (state.DealCollateralBounds, error)
	StateMarketBalance(context.Context, address.Address, types.TipSetKey) (chain.MarketBalance, error)
	StateAccountKey(context.Context, address.Address, types.TipSetKey) (address.Address, error)
	StateListMiners(context.Context, types.TipSetKey) ([]address.Address, error)
	StateMinerInfo(context.Context, address.Address, types.TipSetKey) (miner.MinerInfo, error)
}

type MpoolAPI interface {
	MpoolPushMessage(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec) (*types.SignedMessage, error)
}

type MinerApi interface {
	Address() address.Address
	GetSectorInfo(sid abi.SectorNumber) (sealing.SectorInfo, error)
}

func NewNodeClient(ctx context.Context, cfg *config.NodeConfig) (*NodeClient, jsonrpc.ClientCloser, error) {
	headers := http.Header{}
	if len(cfg.Token) != 0 {
		headers.Add("Authorization", "Bearer "+string(cfg.Token))
	}
	addr, err := DialArgs(cfg.Url)
	if err != nil {
		return nil, nil, err
	}
	var res NodeClient
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", []interface{}{&res}, headers)
	return &res, closer, err
}

func DialArgs(addr string) (string, error) {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err == nil {
		_, addr, err := manet.DialArgs(ma)
		if err != nil {
			return "", err
		}

		return "ws://" + addr + "/rpc/v0", nil
	}

	_, err = url.Parse(addr)
	if err != nil {
		return "", err
	}
	return addr + "/rpc/v0", nil
}
