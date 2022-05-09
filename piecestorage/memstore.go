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
	Name              string
	data              map[string][]byte
	dataLk            *sync.RWMutex
	status            *StorageStatus //status for testing
	RedirectResources map[string]bool
}

func NewMemPieceStore(name string, status *StorageStatus) *MemPieceStore {
	return &MemPieceStore{
		data:              make(map[string][]byte),
		dataLk:            &sync.RWMutex{},
		status:            status,
		Name:              name,
		RedirectResources: make(map[string]bool),
	}
}
func (m *MemPieceStore) Type() Protocol {
	return MemStore
}

func (m *MemPieceStore) SaveTo(ctx context.Context, resourceId string, reader io.Reader) (int64, error) {
	m.dataLk.Lock()
	defer m.dataLk.Unlock()
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	m.data[resourceId] = bytes
	return int64(len(bytes)), nil
}

func (m *MemPieceStore) Len(ctx context.Context, resourceId string) (int64, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[resourceId]; ok {
		return int64(len(data)), nil
	}
	return 0, fmt.Errorf("unable to find resource %resourceId", resourceId)

}

func (m *MemPieceStore) GetReaderCloser(ctx context.Context, resourceId string) (io.ReadCloser, error) {
	return m.GetMountReader(ctx, resourceId)
}

func (m *MemPieceStore) GetMountReader(ctx context.Context, resourceId string) (mount.Reader, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[resourceId]; ok {
		r := bytes.NewReader(data)
		return wraperCloser{r, r}, nil
	}
	return nil, fmt.Errorf("unable to find resource %resourceId", resourceId)

}

func (m *MemPieceStore) Has(ctx context.Context, resourceId string) (bool, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	_, ok := m.data[resourceId]
	return ok, nil
}

func (m *MemPieceStore) CanAllocate(size int64) bool {
	if m.status != nil {
		return m.status.Available > size
	}
	return true
}

func (m *MemPieceStore) GetRedirectUrl(_ context.Context, resourceId string) (string, error) {
	if isRedirect, ok := m.RedirectResources[resourceId]; ok {
		if isRedirect {
			return fmt.Sprintf("mock redirect resourceId %s", resourceId), nil
		}
		return "", ErrUnsupportRedirect
	}
	return "", ErrUnsupportRedirect
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
