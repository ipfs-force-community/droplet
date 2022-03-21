package piecestorage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
)

var _ IPieceStorage = (*MemPieceStore)(nil)

type MemPieceStore struct {
	data   map[string][]byte
	dataLk *sync.RWMutex
}

func NewMemPieceStore() *MemPieceStore {
	return &MemPieceStore{
		data:   make(map[string][]byte),
		dataLk: &sync.RWMutex{},
	}
}
func (m MemPieceStore) Type() Protocol {
	return MemStore
}

func (m MemPieceStore) SaveTo(ctx context.Context, s string, reader io.Reader) (int64, error) {
	m.dataLk.Lock()
	defer m.dataLk.Unlock()
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	m.data[s] = bytes
	return int64(len(bytes)), nil
}

func (m MemPieceStore) Read(ctx context.Context, s string) (io.ReadCloser, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[s]; ok {
		return wraperCloser{bytes.NewReader(data)}, nil
	}
	return nil, fmt.Errorf("unable to find resource %s", s)
}

func (m MemPieceStore) Len(ctx context.Context, s string) (int64, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[s]; ok {
		return int64(len(data)), nil
	}
	return 0, fmt.Errorf("unable to find resource %s", s)

}

func (m MemPieceStore) ReadOffset(ctx context.Context, s string, i int, i2 int) (io.ReadCloser, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	if data, ok := m.data[s]; ok {
		return wraperCloser{bytes.NewReader(data[i:i2])}, nil
	}
	return nil, fmt.Errorf("unable to find resource %s", s)

}

func (m MemPieceStore) Has(ctx context.Context, s string) (bool, error) {
	m.dataLk.RLock()
	defer m.dataLk.RUnlock()
	_, ok := m.data[s]
	return ok, nil
}

func (m MemPieceStore) Validate(s string) error {
	return nil
}

func (m MemPieceStore) GetReadUrl(ctx context.Context, s string) (string, error) {
	return s, nil
}

func (m MemPieceStore) GetWriteUrl(ctx context.Context, s string) (string, error) {
	return s, nil
}

type wraperCloser struct {
	io.Reader
}

func (wraperCloser) Close() error {
	return nil
}
