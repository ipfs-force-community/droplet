package piecestorage

import (
	"github.com/ipfs-force-community/venus-common-utils/builder"

	"github.com/filecoin-project/venus-market/v2/config"
)

var PieceStorageOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		// piece
		builder.Override(new(*PieceStorageManager), func(cfg *config.PieceStorage) (*PieceStorageManager, error) {
			return NewPieceStorageManager(cfg)
		}),
	)
}
