package piecestorage

import (
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/types/market"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/stretchr/testify/assert"
)

func TestFsAddPieceStorage(t *testing.T) {
	psm, err := NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	path := os.TempDir()

	ps, err := NewFsPieceStorage(&config.FsPieceStorage{
		ReadOnly: false,
		Path:     path,
		Name:     "test",
	})
	assert.Nil(t, err)

	err = psm.AddPieceStorage(ps)
	assert.Nil(t, err)

	info := psm.ListStorageInfos()
	assert.Equal(t, 1, len(info.FsStorage))

}

func TestListStorageInfos(t *testing.T) {
	psm, err := NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	path := os.TempDir()
	name := "test"

	ps, err := NewFsPieceStorage(&config.FsPieceStorage{
		ReadOnly: false,
		Path:     path,
		Name:     name,
	})
	assert.Nil(t, err)

	err = psm.AddPieceStorage(ps)
	assert.Nil(t, err)

	info := psm.ListStorageInfos()
	assert.Equal(t, 1, len(info.FsStorage))
}

func TestRmPieceStorage(t *testing.T) {
	psm, err := NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	path := os.TempDir()
	name := "test"

	ps, err := NewFsPieceStorage(&config.FsPieceStorage{
		ReadOnly: false,
		Path:     path,
		Name:     name,
	})
	assert.Nil(t, err)

	err = psm.AddPieceStorage(ps)
	assert.Nil(t, err)

	err = psm.RemovePieceStorage("test2")
	ErrPieceStorageNotFound := fmt.Errorf("storage test2 not exist")

	assert.Equal(t, ErrPieceStorageNotFound, err)

	err = psm.RemovePieceStorage(name)
	assert.Nil(t, err)

	info := psm.ListStorageInfos()
	assert.Equal(t, 0, len(info.FsStorage))
}

func TestRandSelect(t *testing.T) {
	psm, err := NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	psm.AddMemPieceStorage(NewMemPieceStore("1", &market.StorageStatus{
		Capacity:  math.MaxInt64,
		Available: math.MaxInt64,
	}))
	psm.AddMemPieceStorage(NewMemPieceStore("2", &market.StorageStatus{
		Capacity:  math.MaxInt64,
		Available: math.MaxInt64,
	}))
	psm.AddMemPieceStorage(NewMemPieceStore("3", &market.StorageStatus{
		Capacity:  math.MaxInt64,
		Available: math.MaxInt64,
	}))

	psm.AddMemPieceStorage(NewMemPieceStore("4", &market.StorageStatus{
		Capacity:  0,
		Available: 0,
	}))

	var selectName []string
	for i := 0; i < 1000; i++ {
		st, err := psm.FindStorageForWrite(1024 * 1024)
		assert.Nil(t, err)
		selectName = append(selectName, st.(*MemPieceStore).Name)
	}
	assert.Contains(t, selectName, "1")
	assert.Contains(t, selectName, "2")
	assert.Contains(t, selectName, "3")
	assert.NotContains(t, selectName, "4")

	_, err = psm.GetPieceStorageByName("1")
	assert.Nil(t, err)

	_, err = psm.GetPieceStorageByName("10")
	assert.NotNil(t, err)
}

func TestEachStorage(t *testing.T) {
	psm, err := NewPieceStorageManager(&config.PieceStorage{})
	assert.Nil(t, err)
	psm.AddMemPieceStorage(NewMemPieceStore("1", nil))
	psm.AddMemPieceStorage(NewMemPieceStore("2", nil))
	psm.AddMemPieceStorage(NewMemPieceStore("3", nil))

	psm.AddMemPieceStorage(NewMemPieceStore("4", &market.StorageStatus{
		Capacity:  0,
		Available: 0,
	}))

	count := 0
	err = psm.EachPieceStorage(func(s IPieceStorage) error {
		count++
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, 4, count)

	count = 0
	err = psm.EachPieceStorage(func(s IPieceStorage) error {
		count++
		if count == 2 {
			return fmt.Errorf("mock error")
		}
		return nil
	})
	assert.NotNil(t, err)
	assert.Equal(t, 2, count)

}
