package piecestorage

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"golang.org/x/xerrors"
)

var PieceStorageOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		//piece
		builder.Override(new(IPieceStorage), NewPieceStorage), //save read piece data
	)
}

func NewPieceStorage(cfg *config.PieceStorage) (IPieceStorage, error) {
	//todo only use one storage current
	multiEnable := 0
	var storage IPieceStorage
	var err error

	if cfg.Fs.Enable {
		multiEnable++
		storage, err = newFsPieceStorage(cfg.Fs.Path)
	}
	if cfg.S3.Enable {
		multiEnable++
		storage, err = newS3PieceStorage(cfg.S3)
	}
	if multiEnable == 0 {
		return nil, xerrors.New("must config a piece storage ")
	} else if multiEnable > 1 {
		return nil, xerrors.New("can only config one piece storage ")
	}
	return storage, err
}
