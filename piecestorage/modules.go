package piecestorage

import (
	"github.com/filecoin-project/venus-market/config"
	"golang.org/x/xerrors"
)

func NewPieceStorage(pieceStrorageCfg *config.PieceStorage) (IPieceStorage, error) {
	switch pieceStrorageCfg.Type {
	case "local":
		return NewPieceFileStorage(pieceStrorageCfg.Path)
	default:
		return nil, xerrors.Errorf("unsupport piece piecestorage type %s", pieceStrorageCfg.Type)
	}
}
