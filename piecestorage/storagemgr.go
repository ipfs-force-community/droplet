package piecestorage

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/filecoin-project/venus-market/config"
)

type PieceStorageManager struct {
	storages []IPieceStorage
}

func NewPieceStorageManager(cfg *config.PieceStorage) (*PieceStorageManager, error) {
	storages, err := NewPieceStorage(cfg)
	if err != nil {
		return nil, err
	}
	return &PieceStorageManager{storages: storages}, nil
}

func (p *PieceStorageManager) SelectStorageForRead(ctx context.Context, s string) (IPieceStorage, error) {
	var storages []IPieceStorage
	for _, st := range p.storages {
		has, err := st.Has(ctx, s)
		if err != nil {
			log.Warnf("got error while check avaibale in storageg")
			continue
		}
		if has {
			storages = append(storages, st)
		}
	}
	switch len(storages) {
	case 0:
		return nil, fmt.Errorf("unable to find storage for resource %s", s)
	case 1:
		return storages[0], nil
	default:
		return storages[rand.Intn(len(storages))], nil
	}
}

func (p *PieceStorageManager) SelectStorageForWrite(size int64) (IPieceStorage, error) {
	var storages []IPieceStorage
	for _, st := range p.storages {
		if !st.ReadOnly() && st.CanAllocate(size) {
			storages = append(storages, st)
		}
	}
	switch len(storages) {
	case 0:
		return nil, fmt.Errorf("unable to find enough space for size %d", size)
	case 1:
		return storages[0], nil
	default:
		return storages[rand.Intn(len(storages))], nil
	}
}

func (p *PieceStorageManager) AddMemPieceStorage(s IPieceStorage) {
	p.storages = append(p.storages, s)
}

func NewPieceStorage(cfg *config.PieceStorage) ([]IPieceStorage, error) {
	var storages []IPieceStorage

	for _, fsCfg := range cfg.Fs {
		st, err := newFsPieceStorage(fsCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create fs piece storage %w", err)
		}
		storages = append(storages, st)
	}

	for _, s3Cfg := range cfg.S3 {
		st, err := newS3PieceStorage(s3Cfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create fs piece storage %w", err)
		}
		storages = append(storages, st)
	}

	return storages, nil
}
