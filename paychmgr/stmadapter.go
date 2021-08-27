package paychmgr

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"
)

type StateMgrAdapter struct {
	bsstore  blockstore.Blockstore
	fullNode apiface.FullNode
}

func NewStateMgrAdapter(fullNode apiface.FullNode) *StateMgrAdapter {
	return &StateMgrAdapter{bsstore: blockstore.NewAPIBlockstore(fullNode), fullNode: fullNode}
}

func (s StateMgrAdapter) ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	switch addr.Protocol() {
	case address.BLS, address.SECP256K1:
		return addr, nil
	case address.Actor:
		return address.Undef, xerrors.New("cannot resolve actor address to key address")
	default:
	}
	state := state.NewView(cbor.NewCborStore(s.bsstore), ts.ParentState())
	return state.ResolveToKeyAddr(ctx, addr)
}

func (s StateMgrAdapter) GetPaychState(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, paych.State, error) {
	state := state.NewView(cbor.NewCborStore(s.bsstore), ts.ParentState())
	act, err := state.LoadActor(ctx, addr)
	if err != nil {
		return nil, nil, err
	}

	actState, err := state.LoadPaychState(ctx, act)
	if err != nil {
		return nil, nil, err
	}
	return act, actState, nil
}

func (s StateMgrAdapter) Call(ctx context.Context, msg *types.UnsignedMessage, ts *types.TipSet) (*types.InvocResult, error) {
	return s.fullNode.StateCall(ctx, msg, ts.Key())
}

func (s StateMgrAdapter) GetMarketState(ctx context.Context, ts *types.TipSet) (market.State, error) {
	state := state.NewView(cbor.NewCborStore(s.bsstore), ts.ParentState())
	return state.LoadMarketState(ctx)
}
