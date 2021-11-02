package paychmgr

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/blockstore"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/state"
	types2 "github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"
)

// stateManagerAPI defines the methods needed from StateManager
type IStateManager interface {
	resolveToKeyAddress(ctx context.Context, addr address.Address, ts *types2.TipSet) (address.Address, error)
	getPaychState(ctx context.Context, addr address.Address, ts *types2.TipSet) (*types2.Actor, paych.State, error)
	call(ctx context.Context, msg *types2.UnsignedMessage, ts *types2.TipSet) (*types2.InvocResult, error)
	getMarketState(ctx context.Context, ts *types2.TipSet) (market.State, error)
}

type StateMgrAdapter struct {
	bsstore  blockstore.Blockstore
	fullNode apiface.FullNode
}

func newStateMgrAdapter(fullNode apiface.FullNode) IStateManager {
	return &StateMgrAdapter{bsstore: blockstore.NewAPIBlockstore(fullNode), fullNode: fullNode}
}

func (s StateMgrAdapter) resolveToKeyAddress(ctx context.Context, addr address.Address, ts *types2.TipSet) (address.Address, error) {
	switch addr.Protocol() {
	case address.BLS, address.SECP256K1:
		return addr, nil
	case address.Actor:
		return address.Undef, xerrors.New("cannot resolve actor address to key address")
	default:
	}
	var err error
	if ts == nil {
		ts, err = s.fullNode.ChainHead(ctx)
		if err != nil {
			return address.Undef, err
		}
	}
	state := state.NewView(cbor.NewCborStore(s.bsstore), ts.ParentState())
	return state.ResolveToKeyAddr(ctx, addr)
}

func (s StateMgrAdapter) getPaychState(ctx context.Context, addr address.Address, ts *types2.TipSet) (*types2.Actor, paych.State, error) {
	var err error
	if ts == nil {
		ts, err = s.fullNode.ChainHead(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
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

func (s StateMgrAdapter) call(ctx context.Context, msg *types2.UnsignedMessage, ts *types2.TipSet) (*types2.InvocResult, error) {
	var err error
	if ts == nil {
		ts, err = s.fullNode.ChainHead(ctx)
		if err != nil {
			return nil, err
		}
	}
	return s.fullNode.StateCall(ctx, msg, ts.Key())
}

func (s StateMgrAdapter) getMarketState(ctx context.Context, ts *types2.TipSet) (market.State, error) {
	var err error
	if ts == nil {
		ts, err = s.fullNode.ChainHead(ctx)
		if err != nil {
			return nil, err
		}
	}
	state := state.NewView(cbor.NewCborStore(s.bsstore), ts.ParentState())
	return state.LoadMarketState(ctx)
}
