package piecestorage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/filecoin-project/dagstore/mount"
)

var _ IPieceStorage = (*MemPieceStore)(nil)

type MemPieceStore struct {
	data   map[string][]byte
	dataLk *sync.RWMutex
	status *StorageStatus //status for testing
}

func NewMemPieceStore(status *StorageStatus) *MemPieceStore {
	return &MemPieceStore{
		data:   make(map[string][]byte),
		dataLk: &sync.RWMutex{},
		status: status,
	}
}
func (m *MemPieceStore) Type() Protocol {
	return MemStore
}

func (m *MemPieceStore) SaveTo(ctx context.Context, s string, reader io.Reader) (int64, error) {
	m.dataLk.Lock()
	defer m.dataLk.Unlock()
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	m.data[s] = bytes
	return int64(len(bytes)), nil
}

func (m *MemPieceStore) Read(ctx context.Context, s string) (io.ReadCloser, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[s]; ok {
		r := bytes.NewReader(data)
		return wraperCloser{r, r}, nil
	}
	return nil, fmt.Errorf("unable to find resource %s", s)
}

func (m *MemPieceStore) Len(ctx context.Context, s string) (int64, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[s]; ok {
		return int64(len(data)), nil
	}
	return 0, fmt.Errorf("unable to find resource %s", s)

}

func (m *MemPieceStore) GetReaderCloser(ctx context.Context, s string) (io.ReadCloser, error) {
	return m.GetMountReader(ctx, s)
}

func (m *MemPieceStore) GetMountReader(ctx context.Context, s string) (mount.Reader, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[s]; ok {
		r := bytes.NewReader(data)
		return wraperCloser{r, r}, nil
	}
	return nil, fmt.Errorf("unable to find resource %s", s)

}

func (m *MemPieceStore) Has(ctx context.Context, s string) (bool, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	_, ok := m.data[s]
	return ok, nil
}

func (m *MemPieceStore) CanAllocate(size int64) bool {
	if m.status != nil {
		return m.status.Available > size
	}
	return true
}

func (m *MemPieceStore) Validate(s string) error {
	return nil
}

func (m *MemPieceStore) ReadOnly() bool {
	return false
}

type wraperCloser struct {
	io.ReadSeeker
	io.ReaderAt
}

func (wraperCloser) Close() error {
	return nil
}
