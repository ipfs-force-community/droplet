package piecestorage

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/filecoin-project/venus-market/v2/config"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

type PieceStorageManager struct {
	storages []IPieceStorage
}

func NewPieceStorageManager(cfg *config.PieceStorage) (*PieceStorageManager, error) {
	var storages []IPieceStorage
	exist := make(map[string]bool)

	// todo: extract name check logic to a function and check blank in name

	for _, fsCfg := range cfg.Fs {
		// check if storage already exist in manager and it's name is not empty
		if fsCfg.Name == "" {
			return nil, fmt.Errorf("fs piece storage name is empty")
		}
		if exist[fsCfg.Name] {
			return nil, fmt.Errorf("duplicate storage name: %s", fsCfg.Name)
		}
		exist[fsCfg.Name] = true

		st, err := NewFsPieceStorage(fsCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create fs piece storage %w", err)
		}
		storages = append(storages, st)
	}

	for _, s3Cfg := range cfg.S3 {
		// check if storage already exist in manager and it's name is not empty
		if s3Cfg.Name == "" {
			return nil, fmt.Errorf("s3 pieceStorage name is empty")
		}
		if exist[s3Cfg.Name] {
			return nil, fmt.Errorf("duplicate storage name: %s", s3Cfg.Name)
		}
		exist[s3Cfg.Name] = true

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

func (p *PieceStorageManager) AddPieceStorage(s IPieceStorage) error {
	// check if storage already exist in manager and it's name is not empty
	if s.GetName() == "" {
		return fmt.Errorf("storage name is empty")
	}
	for _, st := range p.storages {
		if st.GetName() == s.GetName() {
			return fmt.Errorf("duplicate storage name: %s", s.GetName())
		}
	}

	p.storages = append(p.storages, s)

	return nil
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

func (p *PieceStorageManager) GetStorages() []IPieceStorage {
	return p.storages
}

func (p *PieceStorageManager) RemovePieceStorage(name string) error {
	removed := false

	for i := 0; i < len(p.storages); i++ {
		if name == p.storages[i].GetName() {
			removed = true
			p.storages = append(p.storages[:i], p.storages[i+1:]...)
			break

		}
	}

	if !removed {
		return fmt.Errorf("unable to find storage %s", name)
	}
	return nil
}

func (p *PieceStorageManager) ListStorages() types.PieceStorageList {
	var fs = []types.FsStorage{}
	var s3 = []types.S3Storage{}

	for _, st := range p.storages {
		switch st.Type() {
		case S3:
			cfg := st.(*s3PieceStorage).s3Cfg
			s3 = append(s3, types.S3Storage{
				Name:     cfg.Name,
				EndPoint: cfg.EndPoint,
				ReadOnly: cfg.ReadOnly,
			})

		case FS:
			cfg := st.(*fsPieceStorage).fsCfg
			fs = append(fs, types.FsStorage{
				Name:     cfg.Name,
				Path:     cfg.Path,
				ReadOnly: cfg.ReadOnly,
			})
		}
	}
	return types.PieceStorageList{
		FsStorage: fs,
		S3Storage: s3,
	}
}
