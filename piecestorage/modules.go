package piecestorage

import (
	"github.com/ipfs-force-community/venus-common-utils/builder"

	"github.com/filecoin-project/venus-market/v2/config"
)

var (
	SetupPieceStorageMetricsKey = builder.NextInvoke()
)

var PieceStorageOpts = func(cfg *config.PieceStorage) builder.Option {
	return builder.Options(
		// piece
		builder.Override(new(*PieceStorageManager), func() (*PieceStorageManager, error) {
			return NewPieceStorageManager(cfg)
		}),
	)
}
