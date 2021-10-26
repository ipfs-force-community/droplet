package models

import (
	"context"
	"encoding/json"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	market0 "github.com/filecoin-project/specs-actors/actors/builtin/market"
	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/ipfs/go-cid"
	"io"
)

type mockProvider struct{}

func (m mockProvider) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	return []byte("fake token"), 1024, nil
}

func (m mockProvider) AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	panic("implement me")
}

func (m mockProvider) ReserveFunds(ctx context.Context, wallet, addr address.Address, amt abi.TokenAmount) (cid.Cid, error) {
	panic("implement me")
}

func (m mockProvider) ReleaseFunds(ctx context.Context, addr address.Address, amt abi.TokenAmount) error {
	panic("implement me")
}

func (m mockProvider) GetBalance(ctx context.Context, addr address.Address, tok shared.TipSetToken) (storagemarket.Balance, error) {
	panic("implement me")
}

func (m mockProvider) VerifySignature(ctx context.Context, signature crypto.Signature, signer address.Address, plaintext []byte, tok shared.TipSetToken) (bool, error) {
	panic("implement me")
}

func (m mockProvider) WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, cid.Cid, error) error) error {
	panic("implement me")
}

func (m mockProvider) SignBytes(ctx context.Context, signer address.Address, b []byte) (*crypto.Signature, error) {
	signStr := []byte(`{"Type": 1, "Data": "0Te6VibKM4W0E8cgNFZTgiNXzUqgOZJtCPN1DEp2kClTuzUGVzu/umhCM87o76AEpsMkjpJQGo+S8MYHXQdFTAE="}`)
	sign := &crypto.Signature{}
	return sign, json.Unmarshal(signStr, sign)
}

func (m mockProvider) DealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, isVerified bool) (abi.TokenAmount, abi.TokenAmount, error) {
	panic("implement me")
}

func (m mockProvider) OnDealSectorPreCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, proposal market0.DealProposal, publishCid *cid.Cid, cb storagemarket.DealSectorPreCommittedCallback) error {
	panic("implement me")
}

func (m mockProvider) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, sectorNumber abi.SectorNumber, proposal market0.DealProposal, publishCid *cid.Cid, cb storagemarket.DealSectorCommittedCallback) error {
	panic("implement me")
}

func (m mockProvider) OnDealExpiredOrSlashed(ctx context.Context, dealID abi.DealID, onDealExpired storagemarket.DealExpiredCallback, onDealSlashed storagemarket.DealSlashedCallback) error {
	panic("implement me")
}

func (m mockProvider) PublishDeals(ctx context.Context, deal storagemarket.MinerDeal) (cid.Cid, error) {
	panic("implement me")
}

func (m mockProvider) WaitForPublishDeals(ctx context.Context, mcid cid.Cid, proposal market0.DealProposal) (*storagemarket.PublishDealsWaitResult, error) {
	panic("implement me")
}

func (m mockProvider) OnDealComplete(ctx context.Context, deal storagemarket.MinerDeal, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) (*storagemarket.PackingResult, error) {
	panic("implement me")
}

func (m mockProvider) GetMinerWorkerAddress(ctx context.Context, addr address.Address, tok shared.TipSetToken) (address.Address, error) {
	panic("implement me")
}

func (m mockProvider) GetDataCap(ctx context.Context, addr address.Address, tok shared.TipSetToken) (*verifreg0.DataCap, error) {
	panic("implement me")
}

func (m mockProvider) GetProofType(ctx context.Context, addr address.Address, tok shared.TipSetToken) (abi.RegisteredSealProof, error) {
	panic("implement me")
}

var _ storagemarket.StorageProviderNode = (*mockProvider)(nil)
