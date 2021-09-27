package piece

import (
	"context"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/utils"
	"go.uber.org/fx"
)

// NewProviderPieceStore creates a statestore for storing metadata about pieces
// shared by the piecestorage and retrieval providers
func NewProviderPieceStore(lc fx.Lifecycle, piecestore PieceStore, cidStore CIDStore) (ExtendPieceStore, error) {
	ps := struct {
		PieceStore
		CIDStore
	}{piecestore, cidStore}

	ps.OnReady(utils.ReadyLogger("piecestore"))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return ps.Start(ctx)
		},
	})
	return ps, nil
}

func NewPieceStorage(pieceStrorageCfg *config.PieceStorageString) (IPieceStorage, error) {
	err := CheckValidate(string(*pieceStrorageCfg))
	if err != nil {
		return nil, err
	}
	return &PieceStorage{string(*pieceStrorageCfg)}, nil
}

var PieceOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		//piece
		builder.Override(new(IPieceStorage), NewPieceStorage), //save read peiece data
		builder.Override(new(PieceStore), NewDsPieceStore),
		builder.Override(new(CIDStore), NewDsCidInfoStore),
		builder.Override(new(ExtendPieceStore), NewProviderPieceStore),
		builder.Override(new(piecestore.PieceStore), builder.From(new(ExtendPieceStore))), //save piece metadata(location)   save to metadata /storagemarket
	)
}
