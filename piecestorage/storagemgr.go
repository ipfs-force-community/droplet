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
	var storages []IPieceStorage

	for _, fsCfg := range cfg.Fs {
		st, err := NewFsPieceStorage(fsCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create fs piece storage %w", err)
		}
		storages = append(storages, st)
	}

	for _, s3Cfg := range cfg.S3 {
		st, err := newS3PieceStorage(s3Cfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create object piece storage %w", err)
		}
		storages = append(storages, st)
	}
	return &PieceStorageManager{storages: storages}, nil
}

func (p *PieceStorageManager) FindStorageForRead(ctx context.Context, s string) (IPieceStorage, error) {
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

	if len(storages) == 0 {
		return nil, fmt.Errorf("unable to find piece in storage %s", s)
	}

	return randStorageSelector(storages)
}

func (p *PieceStorageManager) FindStorageForWrite(size int64) (IPieceStorage, error) {
	var storages []IPieceStorage
	for _, st := range p.storages {
		//todo readuce too much check on storage
		if !st.ReadOnly() && st.CanAllocate(size) {
			storages = append(storages, st)
		}
	}

	if len(storages) == 0 {
		return nil, fmt.Errorf("unable to find enough space for size %d", size)
	}
	//todo better to use argorithems base on stroage capacity and usage
	return randStorageSelector(storages)
}

func (p *PieceStorageManager) AddMemPieceStorage(s IPieceStorage) {
	p.storages = append(p.storages, s)
}

func randStorageSelector(storages []IPieceStorage) (IPieceStorage, error) {
	switch len(storages) {
	case 0:
		return nil, fmt.Errorf("given storages is zero")
	case 1:
		return storages[0], nil
	default:
		return storages[rand.Intn(len(storages))], nil
	}
}
