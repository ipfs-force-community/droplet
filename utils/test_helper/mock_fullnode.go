package test_helper

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	v1Mock "github.com/filecoin-project/venus/venus-shared/api/chain/v1/mock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

var _ v1.FullNode = (*MockFullnode)(nil)

type MockFullnode struct {
	v1Mock.MockFullNode
	*testing.T
}

func (m MockFullnode) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	return address.NewIDAddress(1)
}

func (m MockFullnode) StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (types.MinerInfo, error) {
	return types.MinerInfo{}, nil
}

func (m MockFullnode) ChainHead(ctx context.Context) (*types.TipSet, error) {
	addr := address.NewForTestGetter()()
	mockCid, _ := cid.Parse("bafy2bzaceddx2jhct4mvnnhsvbsptvr4gp3ta7jjfhk43ikjdxyubuixav6cw")
	ts, _ := types.NewTipSet([]*types.BlockHeader{{
		Miner:                 addr,
		Ticket:                nil,
		ElectionProof:         nil,
		BeaconEntries:         nil,
		WinPoStProof:          nil,
		Parents:               nil,
		ParentWeight:          big.Int{},
		Height:                0,
		ParentStateRoot:       mockCid,
		ParentMessageReceipts: mockCid,
		Messages:              mockCid,
		BLSAggregate:          nil,
		Timestamp:             0,
		BlockSig:              nil,
		ForkSignaling:         0,
		ParentBaseFee:         abi.TokenAmount{},
	}})
	return ts, nil
}

func (m MockFullnode) ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	return MakeTestTipset(m.T), nil
}

func (m MockFullnode) WalletSign(ctx context.Context, k address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error) {
	signStr := []byte(`{"Type": 1, "Data": "0Te6VibKM4W0E8cgNFZTgiNXzUqgOZJtCPN1DEp2kClTuzUGVzu/umhCM87o76AEpsMkjpJQGo+S8MYHXQdFTAE="}`)
	sign := &crypto.Signature{}
	return sign, json.Unmarshal(signStr, sign)
}
