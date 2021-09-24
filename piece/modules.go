package piece

import (
	"context"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/builder"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/utils"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
	"strings"
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

func NewPieceStorage(pieceStrorageCfg *config.PieceStorage) (IPieceStorage, error) {
	pieceStorage := strings.Split(string(*pieceStrorageCfg), ":")
	if len(pieceStorage) != 2 {
		return nil, xerrors.Errorf("wrong format for piece storage %w", *pieceStrorageCfg)
	}
	switch pieceStorage[0] {
	case "fs":
		return NewPieceFileStorage(pieceStorage[1])
	default:
		return nil, xerrors.Errorf("unsupport piece piecestorage type %s", *pieceStrorageCfg)
	}
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
