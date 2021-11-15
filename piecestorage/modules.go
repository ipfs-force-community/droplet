package piecestorage

import (
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
)

var PieceStorageOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		//piece
		builder.Override(new(IPieceStorage), NewPieceStorage), //save read piece data
	)
}
