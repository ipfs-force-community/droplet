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
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	market7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/models"
	network2 "github.com/filecoin-project/venus-market/network"
	"github.com/filecoin-project/venus-market/piecestorage"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1/mock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

//go:embed testdata/import_deal.json
var importDataJsonString []byte

type dealCase struct {
	Proposal *market.MinerDeal
	Result   bool
}

type testGroud struct {
	DealsOnChain map[abi.DealID]types.MarketDeal
	Cases        []*dealCase
}

func TestStorageProviderImpl_ImportPublishedDeal(t *testing.T) {
	provider := setup(t)
	ctx := context.Background()

	var testGround testGroud
	err := json.Unmarshal(importDataJsonString, &testGround)
	if err != nil {
		t.Error(err)
	}
	for _, c := range testGround.Cases {
		err = provider.ImportPublishedDeal(ctx, *c.Proposal)
		assert.Equal(t, c.Result, err == nil, "%v", err)
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
	ask := &StorageAsk{r.StorageAskRepo(), spn}
	h, err := network2.MockHost(ctx)
	if err != nil {
		t.Error(err)
	}
	dt, err := network2.MockDataTransfer(ctx, h)
	if err != nil {
		t.Error(err)
	}

	homeDir := config.HomeDir("")
	pieceStorage := piecestorage.NewMemPieceStore()
	addrMgr := mockAddrMgr{}

	//todo how to mock dagstore
	provider, err := NewStorageProvider(ask, h, config.DefaultMarketConfig, &homeDir, pieceStorage, dt, spn, nil, r, addrMgr, nil)
	if err != nil {
		t.Error(err)
	}
	return provider
}

type mockAddrMgr struct {
}

func (m mockAddrMgr) Has(ctx context.Context, addr address.Address) bool {
	return addr.String() == "t01043"
}

func (m mockAddrMgr) ActorAddress(ctx context.Context) ([]address.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) ActorList(ctx context.Context) ([]market.User, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) GetMiners(ctx context.Context) ([]market.User, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) GetAccount(ctx context.Context, addr address.Address) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAddrMgr) AddAddress(ctx context.Context, user market.User) error {
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

func (m *mockProviderNode) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
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

func (m *mockProviderNode) VerifySignature(ctx context.Context, signature crypto.Signature, signer address.Address, plaintext []byte, tok shared.TipSetToken) (bool, error) {
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

func (m *mockProviderNode) PublishDeals(ctx context.Context, deal market.MinerDeal) (cid.Cid, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockProviderNode) WaitForPublishDeals(ctx context.Context, mcid cid.Cid, proposal market7.DealProposal) (*storagemarket.PublishDealsWaitResult, error) {
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
