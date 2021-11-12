package piece

import (
	"github.com/filecoin-project/go-fil-markets/piecestore"

	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
)

var PieceOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		//piece
		builder.Override(new(IPieceStorage), NewPieceStorage), //save read piece data
		builder.Override(new(DealAssiger), NewDealAssigner),
		builder.Override(new(piecestore.PieceStore), builder.From(new(DealAssiger))), //save piece metadata(location)   save to metadata /storagemarket
	)
}
