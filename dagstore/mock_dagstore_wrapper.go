package dagstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/blockstore"
	carindex "github.com/ipld/go-car/v2/index"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"
)

type Registration struct {
	CarPath   string
	EagerInit bool
}

// MockDagStoreWrapper is used to mock out the DAG store wrapper operations
// for the tests.
// It simulates getting deal info from a piece store and unsealing the data for
// the deal from a retrieval provider node.
type MockDagStoreWrapper struct {
	lk              sync.Mutex
	registrations   map[cid.Cid]Registration
	piecesWithBlock map[cid.Cid][]cid.Cid
}

var _ stores.DAGStoreWrapper = (*MockDagStoreWrapper)(nil)

func NewMockDagStoreWrapper() *MockDagStoreWrapper {
	return &MockDagStoreWrapper{
		registrations:   make(map[cid.Cid]Registration),
		piecesWithBlock: make(map[cid.Cid][]cid.Cid),
	}
}

func (m *MockDagStoreWrapper) RegisterShard(ctx context.Context, pieceCid cid.Cid, carPath string, eagerInit bool, resch chan dagstore.ShardResult) error {
	m.lk.Lock()
	defer m.lk.Unlock()

	m.registrations[pieceCid] = Registration{
		CarPath:   carPath,
		EagerInit: eagerInit,
	}

	resch <- dagstore.ShardResult{}
	return nil
}

func (m *MockDagStoreWrapper) GetIterableIndexForPiece(c cid.Cid) (carindex.IterableIndex, error) {
	return nil, nil
}

func (m *MockDagStoreWrapper) MigrateDeals(ctx context.Context, deals []storagemarket.MinerDeal) (bool, error) {
	return true, nil
}

func (m *MockDagStoreWrapper) LenRegistrations() int {
	m.lk.Lock()
	defer m.lk.Unlock()

	return len(m.registrations)
}

func (m *MockDagStoreWrapper) GetRegistration(pieceCid cid.Cid) (Registration, bool) {
	m.lk.Lock()
	defer m.lk.Unlock()

	reg, ok := m.registrations[pieceCid]
	return reg, ok
}

func (m *MockDagStoreWrapper) ClearRegistrations() {
	m.lk.Lock()
	defer m.lk.Unlock()

	m.registrations = make(map[cid.Cid]Registration)
}

func (m *MockDagStoreWrapper) LoadShard(ctx context.Context, pieceCid cid.Cid) (stores.ClosableBlockstore, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	pieceInfo, ok := m.registrations[pieceCid]
	if !ok {
		return nil, fmt.Errorf("no shard for piece CID %s", pieceCid)
	}

	fileR, err := os.Open(pieceInfo.CarPath)
	if err != nil {
		return nil, err
	}
	return getBlockstoreFromReader(fileR, pieceCid)
}

func getBlockstoreFromReader(r io.ReadCloser, pieceCid cid.Cid) (stores.ClosableBlockstore, error) {
	// Write the piece to a file
	tmpFile, err := os.CreateTemp("", "dagstoretmp")
	if err != nil {
		return nil, fmt.Errorf("creating temp file for piece CID %s: %w", pieceCid, err)
	}

	_, err = io.Copy(tmpFile, r)
	if err != nil {
		return nil, fmt.Errorf("copying read stream to temp file for piece CID %s: %w", pieceCid, err)
	}

	err = tmpFile.Close()
	if err != nil {
		return nil, fmt.Errorf("closing temp file for piece CID %s: %w", pieceCid, err)
	}

	// Get a blockstore from the CAR file
	return blockstore.OpenReadOnly(tmpFile.Name(), carv2.ZeroLengthSectionAsEOF(true), blockstore.UseWholeCIDs(true))
}

func (m *MockDagStoreWrapper) Close() error {
	return nil
}

func (m *MockDagStoreWrapper) GetPiecesContainingBlock(blockCID cid.Cid) ([]cid.Cid, error) {
	m.lk.Lock()
	defer m.lk.Unlock()

	pieces, ok := m.piecesWithBlock[blockCID]
	if !ok {
		return nil, retrievalmarket.ErrNotFound
	}

	return pieces, nil
}

// Used by the tests to add an entry to the index of block CID -> []piece CID
func (m *MockDagStoreWrapper) AddBlockToPieceIndex(blockCID cid.Cid, pieceCid cid.Cid) {
	m.lk.Lock()
	defer m.lk.Unlock()

	pieces, ok := m.piecesWithBlock[blockCID]
	if !ok {
		m.piecesWithBlock[blockCID] = []cid.Cid{pieceCid}
	} else {
		m.piecesWithBlock[blockCID] = append(pieces, pieceCid)
	}
}
