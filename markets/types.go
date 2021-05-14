package markets

import (
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-statestore"
)

type ProviderDealStore *statestore.StateStore
type ProviderPieceStore piecestore.PieceStore