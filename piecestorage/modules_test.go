package piecestorage

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewPieceStorage(t *testing.T) {
	cfg := config.PieceStorage{
		Fs: config.FsPieceStorage{
			Enable: true,
			Path:   "xxxxx",
		},
		S3:        config.S3PieceStorage{},
		PreSignS3: config.PreSignS3PieceStorage{},
	}

	storage, err := NewPieceStorage(cfg)
	require.NoError(t, err)
	require.Equal(t, storage.Type(), FS)
}

func TestNewPieceStorageFromPointCfg(t *testing.T) {
	cfg := &config.PieceStorage{
		Fs: config.FsPieceStorage{
			Enable: true,
			Path:   "xxxxx",
		},
		S3:        config.S3PieceStorage{},
		PreSignS3: config.PreSignS3PieceStorage{},
	}

	storage, err := NewPieceStorage(cfg)
	require.NoError(t, err)
	require.Equal(t, storage.Type(), FS)
}

func TestNewPieceStorageNoStorageEnable(t *testing.T) {
	cfg := &config.PieceStorage{
		Fs:        config.FsPieceStorage{},
		S3:        config.S3PieceStorage{},
		PreSignS3: config.PreSignS3PieceStorage{},
	}

	_, err := NewPieceStorage(cfg)
	require.Contains(t, err.Error(), "must config a piece storage ")
}

func TestNewPieceStorageMultiStorageEnable(t *testing.T) {
	RegisterPieceStorage(PreSignS3, ProtocolResolver{
		Parser: func(cfg string) (interface{}, error) {
			return config.PreSignS3PieceStorage{Enable: true}, nil
		},
		Constructor: func(cfg interface{}) (IPieceStorage, error) {
			return nil, nil
		},
	})
	cfg := &config.PieceStorage{
		Fs: config.FsPieceStorage{
			Enable: true,
			Path:   "xxxx",
		},
		S3: config.S3PieceStorage{
			Enable: false,
		},
		PreSignS3: config.PreSignS3PieceStorage{
			Enable: true,
		},
	}

	_, err := NewPieceStorage(cfg)
	require.Contains(t, err.Error(), "can only config one piece storage")
}
