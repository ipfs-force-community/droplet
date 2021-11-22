package piecestorage

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs-force-community/venus-common-utils/builder"
)

var PieceStorageOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		//piece
		builder.Override(new(IPieceStorage), NewPieceStorage), //save read piece data
	)
}
