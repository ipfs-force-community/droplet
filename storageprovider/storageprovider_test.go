package storageprovider

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin/v8/market"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/models"
	network2 "github.com/filecoin-project/venus-market/v2/network"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	vCrypto "github.com/filecoin-project/venus/pkg/crypto"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1/mock"
	"github.com/filecoin-project/venus/venus-shared/types"
	marketypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

//go:embed testdata/import_deal.json
var importDataJsonString []byte

type publishdealCase struct {
	Proposal *marketypes.MinerDeal
	Result   bool
}

type importOfflineDealResult int

const (
	success importOfflineDealResult = iota
	dealExists
	minerNotFound
	signatureInvalid
	dealStatusInvalid
	transferTypeInvalid
)

type offlinedealCase struct {
	Proposal *marketypes.MinerDeal
	Result   importOfflineDealResult
}

type testGroud struct {
	DealsOnChain     map[abi.DealID]types.MarketDeal
	PublishDealCases []*publishdealCase
	OfflineDealCase  []*offlinedealCase
}

func TestStorageProviderImpl_ImportPublishedDeal(t *testing.T) {
	provider := setup(t)
	ctx := context.Background()

	var testGround testGroud
	err := json.Unmarshal(importDataJsonString, &testGround)
	if err != nil {
		t.Error(err)
	}
	for _, c := range testGround.PublishDealCases {
		err = provider.ImportPublishedDeal(ctx, *c.Proposal)
		assert.Equal(t, c.Result, err == nil, "ProposalCid: %v, err: %v", c.Proposal.ProposalCid, err)
	}
}

func TestStorageProviderImpl_ImportOfflineDeal(t *testing.T) {
	provider := setup(t)
	ctx := context.Background()

	var testGround testGroud
	err := json.Unmarshal(importDataJsonString, &testGround)
	if err != nil {
		t.Error(err)
	}
	for _, c := range testGround.OfflineDealCase {
		err = provider.ImportOfflineDeal(ctx, *c.Proposal)
		switch c.Result {
		case success:
			assert.NoError(t, err)
			t.Log("Success")
		case dealExists:
			assert.Contains(t, err.Error(), "deal exist")
			t.Logf("DealExists: %v", err)
		case minerNotFound:
			assert.Contains(t, err.Error(), fmt.Sprintf("miner %s not support", c.Proposal.Proposal.Provider.String()))
			t.Logf("MinerNotFound: %v", err)
		case signatureInvalid:
			assert.Contains(t, err.Error(), "verifying StorageDealProposal")
			t.Logf("SignatureInvalid: %v", err)
		case dealStatusInvalid:
			assert.Contains(t, err.Error(), "deal state")
			t.Logf("DealStatusInvalid: %v", err)
		case transferTypeInvalid:
			assert.Contains(t, err.Error(), "transfer type")
			t.Logf("TransferTypeInvalid: %v", err)
		}
	}
}

func setup(t *testing.T) StorageProvider {
	ctx := context.Background()
	spn := newMockProviderNode()

	var testGround testGroud
	err := json.Unmarshal(importDataJsonString, &testGround)
	if err != nil {
		t.Error(err)
	}

	for did, c := range testGround.DealsOnChain {
		spn.addDeal(ctx, did, c)
	}

	r := models.NewInMemoryRepo()
	ask, _ := NewStorageAsk(ctx, r, spn)
	h, err := network2.MockHost(ctx)
	if err != nil {
		t.Error(err)
	}
	dt, err := network2.MockDataTransfer(ctx, h)
	if err != nil {
		t.Error(err)
	}

	homeDir := config.HomeDir("")
	psManager, err := piecestorage.NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	psManager.AddMemPieceStorage(piecestorage.NewMemPieceStore("", nil))
	addrMgr := mockAddrMgr{}

	//todo how to mock dagstore
	provider, err := NewStorageProvider(ctx, ask, h, config.DefaultMarketConfig, &homeDir, psManager, dt, spn, nil, r, addrMgr, nil)
	if err != nil {
		t.Error(err)
	}
	return provider
}

type mockAddrMgr struct {
}

func (m mockAddrMgr) Has(ctx context.Context, addr address.Address) bool {
	return addr.String() == "t01043" || addr.String() == "t010938"
}

func (m mockAddrMgr) ActorAddress(ctx context.Context) ([]address.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) ActorList(ctx context.Context) ([]marketypes.User, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) GetMiners(ctx context.Context) ([]marketypes.User, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) GetAccount(ctx context.Context, addr address.Address) (string, error) {
	//TODO implement me
	panic("implement me")
}

type mockProviderNode struct {
	mock.MockFullNode
	dataLk sync.Mutex
	data   map[abi.DealID]types.MarketDeal
	head   *types.TipSet
}

func newMockProviderNode() *mockProviderNode {
	return &mockProviderNode{
		MockFullNode: mock.MockFullNode{},
		dataLk:       sync.Mutex{},
		data:         make(map[abi.DealID]types.MarketDeal),
		head:         nil,
	}
}

func (m *mockProviderNode) addDeal(ctx context.Context, dealID abi.DealID, deal types.MarketDeal) {
	m.dataLk.Lock()
	defer m.dataLk.Unlock()
	m.data[dealID] = deal
}

func (m *mockProviderNode) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*types.MarketDeal, error) {
	m.dataLk.Lock()
	defer m.dataLk.Unlock()
	if marketDeal, ok := m.data[dealID]; ok {
		return &marketDeal, nil
	}
	return nil, fmt.Errorf("unable to find deal %d", dealID)
}

func (m *mockProviderNode) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	head, err := m.ChainHead(ctx)
	if err != nil {
		return nil, 0, err
	}

	return head.Key().Bytes(), head.Height(), nil
}

func (m *mockProviderNode) VerifySignature(ctx context.Context, signature crypto.Signature, signer address.Address, plaintext []byte, tok shared.TipSetToken) (bool, error) {
	err := vCrypto.Verify(&signature, signer, plaintext)
	return err == nil, err
}

func (m *mockProviderNode) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return m.head, nil
}

func (m *mockProviderNode) Sign(ctx context.Context, data interface{}) (*crypto.Signature, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) SignWithGivenMiner(mAddr address.Address) network.ResigningFunc {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) ReserveFunds(ctx context.Context, wallet, addr address.Address, amt abi.TokenAmount) (cid.Cid, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) ReleaseFunds(ctx context.Context, addr address.Address, amt abi.TokenAmount) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) GetBalance(ctx context.Context, addr address.Address, tok shared.TipSetToken) (storagemarket.Balance, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, cid.Cid, error) error) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) DealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, isVerified bool) (abi.TokenAmount, abi.TokenAmount, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) PublishDeals(ctx context.Context, deal marketypes.MinerDeal) (cid.Cid, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) WaitForPublishDeals(ctx context.Context, mcid cid.Cid, proposal market.DealProposal) (*storagemarket.PublishDealsWaitResult, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) GetMinerWorkerAddress(ctx context.Context, addr address.Address, tok shared.TipSetToken) (address.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) GetDataCap(ctx context.Context, addr address.Address, tok shared.TipSetToken) (*abi.StoragePower, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) GetProofType(ctx context.Context, addr address.Address, tok shared.TipSetToken) (abi.RegisteredSealProof, error) {
	//TODO implement me
	panic("implement me")
}
