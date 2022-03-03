package api

import (
	"golang.org/x/xerrors"

	marketapi "github.com/filecoin-project/venus/venus-shared/api/market"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
)

//mock for gen
var _ = xerrors.New("") // nolint

type MarketFullNode = marketapi.IMarket
type MarketFullStruct = marketapi.IMarketStruct

type MarketClientNode = clientapi.IMarketClient
type MarketClientStruct = clientapi.IMarketClientStruct
