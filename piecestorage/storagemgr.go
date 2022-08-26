package piecestorage

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/filecoin-project/venus-market/v2/config"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

type PieceStorageManager struct {
	lk       sync.RWMutex
	storages map[string]IPieceStorage
}

func NewPieceStorageManager(cfg *config.PieceStorage) (*PieceStorageManager, error) {
	var storages = make(map[string]IPieceStorage)

	// todo: extract name check logic to a function and check blank in name

	for _, fsCfg := range cfg.Fs {
		// check if storage already exist in manager and it's name is not empty
		if fsCfg.Name == "" {
			return nil, fmt.Errorf("fs piece storage name is empty, must set storage name in piece storage config `name=yourname`")
		}
		_, ok := storages[fsCfg.Name]
		if ok {
			return nil, fmt.Errorf("duplicate storage name: %s", fsCfg.Name)
		}

		st, err := NewFsPieceStorage(fsCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create fs piece storage %w", err)
		}
		storages[fsCfg.Name] = st
	}

	for _, s3Cfg := range cfg.S3 {
		// check if storage already exist in manager and it's name is not empty
		if s3Cfg.Name == "" {
			return nil, fmt.Errorf("s3 pieceStorage name is empty, must set storage name in piece storage config `name=yourname`")
		}
		_, ok := storages[s3Cfg.Name]
		if ok {
			return nil, fmt.Errorf("duplicate storage name: %s", s3Cfg.Name)
		}

		st, err := NewS3PieceStorage(s3Cfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create object piece storage %w", err)
		}
		storages[s3Cfg.Name] = st
	}
	return &PieceStorageManager{
		lk:       sync.RWMutex{},
		storages: storages,
	}, nil
}

func (p *PieceStorageManager) FindStorageForRead(ctx context.Context, s string) (IPieceStorage, error) {
	var storages []IPieceStorage
	_ = p.EachPieceStorage(func(st IPieceStorage) error {
		has, err := st.Has(ctx, s)
		if err != nil {
			log.Warnf("got error while check avaibale in storageg")
			return nil
		}
		if has {
			storages = append(storages, st)
		}
		return nil
	})

	if len(storages) == 0 {
		return nil, fmt.Errorf("unable to find piece in storage %s", s)
	}

	return randStorageSelector(storages)
}

func (p *PieceStorageManager) FindStorageForWrite(size int64) (IPieceStorage, error) {
	var storages []IPieceStorage
	_ = p.EachPieceStorage(func(st IPieceStorage) error {
		if st.ReadOnly() {
			return nil
		}
		storageSt, err := st.GetStorageStatus()
		if err != nil {
			log.Errorf("get available bytes from storage(%s)", st.GetName())
			return nil
		}
		if storageSt.Available > size {
			storages = append(storages, st)
		}
		return nil
	})

	if len(storages) == 0 {
		return nil, fmt.Errorf("unable to select a piece storage that have enough space for piece(%d)", size)
	}
	//todo better to use argorithems base on stroage capacity and usage
	return randStorageSelector(storages)
}

func (p *PieceStorageManager) GetPieceStorageByName(name string) (IPieceStorage, error) {
	p.lk.Lock()
	defer p.lk.Unlock()

	if storage, ok := p.storages[name]; ok {
		return storage, nil
	}
	return nil, fmt.Errorf("storage %s not exit please check config", name)
}

func (p *PieceStorageManager) AddMemPieceStorage(s IPieceStorage) {
	p.lk.Lock()
	defer p.lk.Unlock()

	p.storages[s.GetName()] = s
}

func (p *PieceStorageManager) AddPieceStorage(s IPieceStorage) error {
	p.lk.Lock()
	defer p.lk.Unlock()

	// check if storage already exist in manager and it's name is not empty
	_, ok := p.storages[s.GetName()]
	if ok {
		return fmt.Errorf("duplicate storage name: %s", s.GetName())
	}
	p.storages[s.GetName()] = s
	return nil
}

func (p *PieceStorageManager) EachPieceStorage(fn func(IPieceStorage) error) error {
	p.lk.Lock()
	defer p.lk.Unlock()

	for _, s := range p.storages {
		if err := fn(s); err != nil {
			return err
		}
	}
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

func (p *PieceStorageManager) RemovePieceStorage(name string) error {
	p.lk.Lock()
	defer p.lk.Unlock()

	_, exist := p.storages[name]
	if !exist {
		return fmt.Errorf("storage %s not exist", name)
	}
	delete(p.storages, name)
	return nil
}

func (p *PieceStorageManager) ListStorageInfos() types.PieceStorageInfos {
	var fs []types.FsStorage
	var s3 []types.S3Storage

	_ = p.EachPieceStorage(func(st IPieceStorage) error {
		status, err := st.GetStorageStatus()
		if err != nil {
			log.Errorf("get storage status failed")
			return nil
		}
		switch st.Type() {
		case S3:
			cfg := st.(*s3PieceStorage).s3Cfg
			s3 = append(s3, types.S3Storage{
				Name:     cfg.Name,
				EndPoint: cfg.EndPoint,
				ReadOnly: cfg.ReadOnly,
				Bucket:   cfg.Bucket,
				SubDir:   cfg.SubDir,
				Status:   status,
			})

		case FS:
			cfg := st.(*fsPieceStorage).fsCfg
			fs = append(fs, types.FsStorage{
				Name:     cfg.Name,
				Path:     cfg.Path,
				ReadOnly: cfg.ReadOnly,
				Status:   status,
			})
		}
		return nil
	})

	return types.PieceStorageInfos{
		FsStorage: fs,
		S3Storage: s3,
	}
}
